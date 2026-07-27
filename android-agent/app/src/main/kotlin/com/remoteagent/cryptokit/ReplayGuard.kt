package com.remoteagent.cryptokit

/**
 * Monotonic per-direction sequence replay protection.
 */
class ReplayGuard {
    private var lastSeq: Long = 0
    private val seen = HashSet<Long>()

    @Synchronized
    fun accept(seq: Long) {
        require(seq > 0) { "sequence must be > 0" }
        require(seq !in seen) { "duplicate sequence" }
        require(seq > lastSeq) { "replayed or older sequence" }
        seen.add(seq)
        lastSeq = seq
        if (seen.size > 4096) {
            seen.clear()
            seen.add(seq)
        }
    }

    @Synchronized
    fun reset() {
        lastSeq = 0
        seen.clear()
    }
}
