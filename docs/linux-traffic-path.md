## Linux Network Namespace Traffic Path

### Egress Path (Process → Wire)

**Starting point:** A process inside a network namespace calls `send()` / `write()` / `sendmsg()`

```
1. System Call Layer
   └─ sys_sendmsg() → sock_sendmsg()

2. Socket Layer (L4)
   └─ TCP/UDP/RAW protocol handler
      ├─ tcp_sendmsg() builds sk_buff (skb)
      └─ Routes lookup via ip_route_output()

3. IP Output (L3) — netfilter hooks begin here
   └─ ip_local_out()
      ├─ [HOOK] Netfilter: NF_INET_LOCAL_OUT        ← nftables output chain
      │         (iptables OUTPUT / nft output)
      └─ ip_output()
         ├─ Fragmentation if needed (ip_fragment)
         └─ [HOOK] Netfilter: NF_INET_POST_ROUTING   ← nftables postrouting chain
                   (iptables POSTROUTING / nft postrouting)
                   (NAT/masquerade happens here)

4. Neighbour Subsystem / ARP
   └─ dst_neigh_output() → neigh_resolve_output()
      └─ ARP resolution, skb held if unresolved

5. Driver Queue (qdisc) — TC lives here
   └─ dev_queue_xmit()
      ├─ [ATTACH] TC egress (sch_handle_egress)      ← tc BPF / cls_bpf / flower
      │           BPF prog type: BPF_PROG_TYPE_SCHED_CLS or _ACT
      │           Runs BEFORE qdisc enqueue
      ├─ Qdisc enqueue (HTB, FQ, pfifo_fast, etc.)
      ├─ [XDP — not applicable on egress from namespace]
      └─ netdev_start_xmit() → driver → NIC

6. (Optional) If veth/bridge/tunnel:
   └─ Packet crosses into another namespace or bridge
      └─ Bridge: ebtables / nftables bridge hooks apply
```

---

### Ingress Path (Wire → Process)

**Starting point:** NIC receives a frame, raises IRQ or NAPI poll

```
1. NIC / Driver
   └─ NAPI poll → netif_receive_skb()
      ├─ [ATTACH] XDP (BPF_PROG_TYPE_XDP)            ← earliest possible intercept
      │           Modes: native (driver), offloaded (NIC), generic (skb)
      │           Actions: XDP_PASS, XDP_DROP, XDP_TX, XDP_REDIRECT
      └─ skb handed to network stack

2. TC Ingress — immediately after XDP
   └─ sch_handle_ingress()
      └─ [ATTACH] TC ingress qdisc (clsact)           ← tc BPF ingress
                  BPF_PROG_TYPE_SCHED_CLS
                  Actions: TC_ACT_OK, TC_ACT_SHOT, TC_ACT_REDIRECT

3. VLAN / GRO / RPS processing
   └─ __netif_receive_skb_core()
      └─ Protocol demux (ETH_P_IP, ETH_P_IPV6, ...)

4. IP Input (L3) — netfilter hooks
   └─ ip_rcv()
      └─ [HOOK] Netfilter: NF_INET_PRE_ROUTING        ← nftables prerouting chain
                (iptables PREROUTING / nft prerouting)
                (DNAT happens here)

5. Routing Decision
   └─ ip_rcv_finish() → ip_route_input()
      ├─ Local delivery → continue down
      └─ Forward → jump to FORWARD path (see below)

6. Local Delivery
   └─ ip_local_deliver()
      ├─ Reassembly if fragmented
      └─ [HOOK] Netfilter: NF_INET_LOCAL_IN           ← nftables input chain
                (iptables INPUT / nft input)

7. Transport Layer (L4)
   └─ TCP/UDP handler
      └─ sk_buff placed in socket receive buffer

8. Socket / Application
   └─ Process wakes, calls recv() / read()
```

---

### Forwarding Path (for traffic transiting the namespace/host)

```
After PRE_ROUTING and routing decision to forward:

   [HOOK] Netfilter: NF_INET_FORWARD                  ← nftables forward chain
          (iptables FORWARD / nft forward)
          (conntrack, policy filtering)
   └─ ip_forward() → ip_output()
      └─ [HOOK] NF_INET_POST_ROUTING                   ← nftables postrouting
         └─ → Egress path step 4 onwards
```

---

### Attachment Point Summary Table

| Subsystem | Location in Path | Direction | Hook / Prog Type | Typical Use |
|---|---|---|---|---|
| **XDP** | Before skb allocation | Ingress only | `BPF_PROG_TYPE_XDP` | DDoS drop, fast redirect, load balancing |
| **TC ingress** | After XDP, before L3 | Ingress | `BPF_PROG_TYPE_SCHED_CLS` | Filtering, redirect, policy |
| **nftables prerouting** | After TC ingress, L3 entry | Ingress | nftables / iptables | DNAT, conntrack init |
| **nftables input** | After routing → local | Ingress | nftables / iptables | Firewall, rate limiting |
| **nftables forward** | After routing → forward | Both | nftables / iptables | Forwarding policy |
| **nftables output** | L3 local out | Egress | nftables / iptables | Output policy, SNAT |
| **nftables postrouting** | After output, before qdisc | Egress | nftables / iptables | Masquerade, SNAT |
| **TC egress** | Before qdisc enqueue | Egress | `BPF_PROG_TYPE_SCHED_CLS` | Shaping, marking, redirect |
| **Qdisc** | After TC egress | Egress | HTB, FQ, etc. | Bandwidth shaping, scheduling |

---

### Key Design Notes

- **XDP is the earliest and fastest** intercept — it runs before skb allocation in native mode, making it ideal for high-throughput drop/redirect scenarios.
- **TC BPF straddles both directions** using the `clsact` qdisc, and unlike XDP it has full skb access and can modify headers freely in both directions.
- **nftables/iptables hooks are netfilter hooks** — nftables is the modern frontend to the same hook infrastructure. They are entirely L3+ and never see raw L2 frames.
- **Conntrack** (connection tracking) is initialized in `NF_INET_PRE_ROUTING` and `NF_INET_LOCAL_OUT`, so all subsequent hooks can match on connection state.
- **NAT** (DNAT at prerouting, SNAT/masquerade at postrouting) is implemented as conntrack extensions evaluated inside netfilter hooks.
- **Bridge traffic** gets its own parallel hook family (`nftables bridge` / `ebtables`) running at L2 before packets are re-injected into the L3 path.
- Within a **veth pair** (common in container networking), TC egress on one end appears as TC ingress on the peer — this is the standard Cilium/Kubernetes dataplane trick for enforcing policy between namespaces without going through the full host stack.
