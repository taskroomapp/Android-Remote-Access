package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import android.location.Criteria
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONObject
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * Location / site fix commands (fresh GPS/network + last-known fallback).
 */
class LocationCommands(private val context: Context) {

    fun getLocation(command: DeviceCommand): CommandResult {
        val fine = context.hasPermission(Manifest.permission.ACCESS_FINE_LOCATION)
        val coarse = context.hasPermission(Manifest.permission.ACCESS_COARSE_LOCATION)
        if (!fine && !coarse) {
            return CommandResult.failed(
                "PERMISSION_DENIED",
                "Location permission not granted. Open the agent app and allow Location access."
            )
        }

        return try {
            val locationManager = context.getSystemService(Context.LOCATION_SERVICE) as? LocationManager
                ?: return CommandResult.failed("Location service not available")

            var gpsOn = false
            var networkOn = false
            try {
                gpsOn = locationManager.isProviderEnabled(LocationManager.GPS_PROVIDER)
                networkOn = locationManager.isProviderEnabled(LocationManager.NETWORK_PROVIDER)
            } catch (_: Exception) {
            }

            val locResult = requestFreshLocation(locationManager, fine)
            if (locResult.location == null) {
                return if (!gpsOn && !networkOn) {
                    CommandResult.failed(
                        "LOCATION_DISABLED",
                        "GPS/network location is off. Enable Location in system settings (or set a mock location on the emulator)."
                    )
                } else {
                    CommandResult.failed(
                        "LOCATION_UNAVAILABLE",
                        "No GPS fix yet. On emulators: Extended controls → Location → set a point and Send. On devices: go outdoors or wait for a GPS lock."
                    )
                }
            }

            val location = locResult.location
            CommandResult.success(
                JSONObject().apply {
                    put("latitude", location.latitude)
                    put("longitude", location.longitude)
                    put("altitude", location.altitude)
                    put("accuracy", location.accuracy)
                    put("timestamp", location.time)
                    put("provider", location.provider ?: "")
                    put("stale", locResult.stale)
                }.toString()
            )
        } catch (e: SecurityException) {
            CommandResult.failed("PERMISSION_DENIED", "Location permission denied: ${e.message}")
        } catch (e: Exception) {
            CommandResult.failed("Failed to get location: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.GET_LOCATION] = CommandExecutor { getLocation(it) }
    }

    private data class LocationResult(
        val location: Location?,
        val stale: Boolean
    )

    private fun requestFreshLocation(locationManager: LocationManager, fine: Boolean): LocationResult {
        val best = pickBestLastKnown(locationManager)

        val latch = CountDownLatch(1)
        val holder = arrayOfNulls<Location>(1)
        val listeners = ArrayList<LocationListener>()

        val mainHandler = Handler(Looper.getMainLooper())
        mainHandler.post {
            try {
                val listener = object : LocationListener {
                    override fun onLocationChanged(loc: Location) {
                        synchronized(holder) {
                            val current = holder[0]
                            if (current == null
                                || loc.accuracy <= 0f
                                || (current.accuracy > 0f && loc.accuracy < current.accuracy)
                                || loc.time >= current.time
                            ) {
                                holder[0] = loc
                            }
                        }
                        latch.countDown()
                    }

                    @Deprecated("Deprecated in Java")
                    override fun onStatusChanged(provider: String?, status: Int, extras: Bundle?) {}

                    override fun onProviderEnabled(provider: String) {}
                    override fun onProviderDisabled(provider: String) {}
                }
                listeners.add(listener)

                var requested = false
                val providers = if (fine) {
                    arrayOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)
                } else {
                    arrayOf(LocationManager.NETWORK_PROVIDER)
                }

                for (provider in providers) {
                    try {
                        if (locationManager.isProviderEnabled(provider)) {
                            locationManager.requestLocationUpdates(
                                provider, 0L, 0f, listener, Looper.getMainLooper()
                            )
                            requested = true
                        }
                    } catch (_: IllegalArgumentException) {
                    } catch (_: SecurityException) {
                    }
                }

                if (!requested) {
                    try {
                        val criteria = Criteria().apply {
                            accuracy = if (fine) Criteria.ACCURACY_FINE else Criteria.ACCURACY_COARSE
                            isCostAllowed = true
                        }
                        val bestProvider = locationManager.getBestProvider(criteria, true)
                        if (bestProvider != null) {
                            locationManager.requestLocationUpdates(
                                bestProvider, 0L, 0f, listener, Looper.getMainLooper()
                            )
                            requested = true
                        }
                    } catch (_: Exception) {
                    }
                }

                if (!requested) {
                    latch.countDown()
                }
            } catch (_: SecurityException) {
                latch.countDown()
            }
        }

        try {
            latch.await(15, TimeUnit.SECONDS)
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
        }

        mainHandler.post {
            for (listener in listeners) {
                try {
                    locationManager.removeUpdates(listener)
                } catch (_: Exception) {
                }
            }
        }

        val fresh: Location?
        synchronized(holder) {
            fresh = holder[0]
        }
        if (fresh != null) {
            return LocationResult(fresh, false)
        }
        val after = pickBestLastKnown(locationManager)
        if (after != null) {
            val stale = !(best == null || after.time > best.time)
            return LocationResult(after, stale)
        }
        if (best != null) {
            return LocationResult(best, true)
        }
        return LocationResult(null, false)
    }

    private fun pickBestLastKnown(locationManager: LocationManager): Location? {
        var best: Location? = null
        val providers = arrayOf(
            LocationManager.GPS_PROVIDER,
            LocationManager.NETWORK_PROVIDER,
            LocationManager.PASSIVE_PROVIDER
        )
        for (provider in providers) {
            try {
                val candidate = locationManager.getLastKnownLocation(provider) ?: continue
                val current = best
                if (current == null
                    || candidate.time > current.time
                    || (candidate.time == current.time && candidate.accuracy < current.accuracy)
                ) {
                    best = candidate
                }
            } catch (_: SecurityException) {
            } catch (_: IllegalArgumentException) {
            }
        }
        return best
    }
}
