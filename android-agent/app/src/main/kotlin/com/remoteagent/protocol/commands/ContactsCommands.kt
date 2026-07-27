package com.remoteagent.protocol.commands

import android.Manifest
import android.content.Context
import android.provider.ContactsContract
import com.remoteagent.protocol.CommandResult
import com.remoteagent.protocol.DeviceCommand
import org.json.JSONArray
import org.json.JSONObject

/**
 * Contacts provider commands.
 */
class ContactsCommands(private val context: Context) {

    fun getContacts(command: DeviceCommand): CommandResult {
        if (!context.hasPermission(Manifest.permission.READ_CONTACTS)) {
            return CommandResult.failed("PERMISSION_DENIED", "READ_CONTACTS permission not granted")
        }

        return try {
            val contacts = JSONArray()
            val resolver = context.contentResolver

            resolver.query(
                ContactsContract.Contacts.CONTENT_URI,
                null, null, null, null
            )?.use { cursor ->
                while (cursor.moveToNext()) {
                    val id = cursor.getString(cursor.getColumnIndexOrThrow(ContactsContract.Contacts._ID))
                    val name = cursor.getString(cursor.getColumnIndexOrThrow(ContactsContract.Contacts.DISPLAY_NAME))

                    val contact = JSONObject().apply {
                        put("id", id)
                        put("name", name ?: "Unknown")
                    }

                    val phones = JSONArray()
                    resolver.query(
                        ContactsContract.CommonDataKinds.Phone.CONTENT_URI,
                        null,
                        "${ContactsContract.CommonDataKinds.Phone.CONTACT_ID} = ?",
                        arrayOf(id),
                        null
                    )?.use { phoneCursor ->
                        while (phoneCursor.moveToNext()) {
                            val phoneNumber = phoneCursor.getString(
                                phoneCursor.getColumnIndexOrThrow(ContactsContract.CommonDataKinds.Phone.NUMBER)
                            )
                            val phoneType = phoneCursor.getString(
                                phoneCursor.getColumnIndexOrThrow(ContactsContract.CommonDataKinds.Phone.TYPE)
                            )
                            phones.put(JSONObject().apply {
                                put("number", phoneNumber)
                                put(
                                    "type",
                                    ContactsContract.CommonDataKinds.Phone.getTypeLabel(
                                        context.resources,
                                        (phoneType ?: "0").toInt(),
                                        "Other"
                                    )
                                )
                            })
                        }
                    }
                    contact.put("phones", phones)

                    val emails = JSONArray()
                    resolver.query(
                        ContactsContract.CommonDataKinds.Email.CONTENT_URI,
                        null,
                        "${ContactsContract.CommonDataKinds.Email.CONTACT_ID} = ?",
                        arrayOf(id),
                        null
                    )?.use { emailCursor ->
                        while (emailCursor.moveToNext()) {
                            val emailAddress = emailCursor.getString(
                                emailCursor.getColumnIndexOrThrow(ContactsContract.CommonDataKinds.Email.ADDRESS)
                            )
                            val emailType = emailCursor.getString(
                                emailCursor.getColumnIndexOrThrow(ContactsContract.CommonDataKinds.Email.TYPE)
                            )
                            emails.put(JSONObject().apply {
                                put("address", emailAddress)
                                put(
                                    "type",
                                    ContactsContract.CommonDataKinds.Email.getTypeLabel(
                                        context.resources,
                                        (emailType ?: "0").toInt(),
                                        "Other"
                                    )
                                )
                            })
                        }
                    }
                    contact.put("emails", emails)
                    contacts.put(contact)
                }
            }

            CommandResult.success(
                JSONObject().apply {
                    put("contacts", contacts)
                    put("count", contacts.length())
                }.toString()
            )
        } catch (e: Exception) {
            CommandResult.failed("Failed to get contacts: ${e.message}")
        }
    }

    fun register(into: MutableMap<DeviceCommand.CommandType, CommandExecutor>) {
        into[DeviceCommand.CommandType.GET_CONTACTS] = CommandExecutor { getContacts(it) }
    }
}
