// dials the go runner printed by `go run ./interop` with js-libp2p's webrtc transport,
// echoes a payload larger than the relay's limits and checks the connection is not relayed.
// usage: node dial.mjs <target multiaddr>
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { circuitRelayTransport } from '@libp2p/circuit-relay-v2'
import { identify } from '@libp2p/identify'
import { tcp } from '@libp2p/tcp'
import { webRTC } from '@libp2p/webrtc'
import { multiaddr } from '@multiformats/multiaddr'
import { createLibp2p } from 'libp2p'
import { randomBytes } from 'node:crypto'

const target = multiaddr(process.argv[2])
const size = Number(process.env.P2PWEBRTC_PAYLOAD ?? 1 << 20)

const node = await createLibp2p({
  transports: [tcp(), webRTC(), circuitRelayTransport()],
  connectionEncrypters: [noise()],
  streamMuxers: [yamux()],
  services: { identify: identify() },
  connectionGater: { denyDialMultiaddr: () => false }
})

try {
  const conn = await node.dial(target, { signal: AbortSignal.timeout(30_000) })
  console.log('connected', conn.remoteAddr.toString(), 'limits', JSON.stringify(conn.limits ?? null))
  if (conn.limits != null) throw new Error('connection is limited, it is the relayed connection')
  if (!conn.remoteAddr.toString().includes('/webrtc')) throw new Error('not a webrtc connection')

  const stream = await conn.newStream('/p2pwebrtc/test/echo', { signal: AbortSignal.timeout(10_000) })
  const payload = randomBytes(size)

  const received = (async () => {
    const chunks = []
    let n = 0
    for await (const chunk of stream) {
      const b = chunk.subarray()
      chunks.push(Buffer.from(b))
      n += b.byteLength
    }
    return Buffer.concat(chunks, n)
  })()

  for (let i = 0; i < payload.length; i += 8192) {
    if (!stream.send(payload.subarray(i, i + 8192))) {
      await stream.onDrain({ signal: AbortSignal.timeout(10_000) })
    }
  }
  await stream.close({ signal: AbortSignal.timeout(10_000) })

  const echoed = await received
  if (!payload.equals(echoed)) throw new Error(`echo mismatch ${payload.length} != ${echoed.length}`)
  console.log('echoed', echoed.length, 'bytes over', conn.remoteAddr.toString())
} finally {
  await node.stop()
}
