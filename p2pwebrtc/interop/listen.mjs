// a js-libp2p echo server reachable only through the go relay, for the go dialer side of the interop.
// prints its /p2p-circuit/webrtc address and serves until killed.
// usage: node listen.mjs <relay multiaddr>
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { circuitRelayTransport } from '@libp2p/circuit-relay-v2'
import { identify } from '@libp2p/identify'
import { tcp } from '@libp2p/tcp'
import { webRTC } from '@libp2p/webrtc'
import { multiaddr } from '@multiformats/multiaddr'
import { createLibp2p } from 'libp2p'

const relay = multiaddr(process.argv[2])

const node = await createLibp2p({
  addresses: { listen: [`${relay.toString()}/p2p-circuit`, '/webrtc'] },
  transports: [tcp(), webRTC(), circuitRelayTransport()],
  connectionEncrypters: [noise()],
  streamMuxers: [yamux()],
  services: { identify: identify() },
  connectionGater: { denyDialMultiaddr: () => false }
})

await node.handle('/p2pwebrtc/test/echo', async (stream, connection) => {
  console.error('echo stream', connection.remoteAddr.toString(), 'limits', JSON.stringify(connection.limits ?? null))
  for await (const chunk of stream) {
    if (!stream.send(chunk)) await stream.onDrain()
  }
  await stream.close()
})

const deadline = Date.now() + 30_000
let addr
while (addr == null && Date.now() < deadline) {
  addr = node.getMultiaddrs().find(a => a.toString().includes('/p2p-circuit/webrtc'))
  if (addr == null) await new Promise(resolve => setTimeout(resolve, 100))
}
if (addr == null) throw new Error('no /p2p-circuit/webrtc address')
console.log(JSON.stringify({ target: addr.toString() }))

process.on('SIGTERM', () => { node.stop().then(() => process.exit(0)) })
