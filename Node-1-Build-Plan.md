# Node #1 — "The Single Hearth" Build Plan

**What this is:** Step 1 of the staged plan in *The Grove & the Trees* (§10) — "Maintain Node #1 operational on the local desk, validating offline storage and private utility without ongoing SaaS costs." This is the seed. Everything else in the plan grows from it.

**Hardware on hand (verified):**
- Dell OptiPlex 7060 Tower (refurb, eBay): 8th-gen Intel Core (i3/i5/i7, 6-core typical), DDR4 (typically 8GB installed, up to 64GB across 4 slots), TPM 2.0, Gigabit Ethernet, USB-C front, ~260W PSU. Bays: 1× internal 3.5", 2× internal 2.5", plus an M.2 slot.
- Micron 1100 256GB 2.5" SATA SSD (refurb, eBay): the boot drive. Fits either 2.5" bay.

## Phase 0 — Inventory and physical build (~30 min)
1. Open the tower and note what's actually inside: CPU model, RAM amount, whether a drive already shipped in it. (Refurb units vary — confirm before assuming.)
2. Mount the Micron 1100 in an empty 2.5" bay; connect SATA data + power. If a spinning HDD came with the unit, keep it as secondary storage or set it aside.
3. Plug in monitor, keyboard, Ethernet (wired — the node should live on Ethernet, not Wi-Fi).
4. Boot into BIOS (F2), confirm the SSD is detected, note the service tag.

## Phase 1 — Operating system (~45 min)
1. On your main computer, flash Ubuntu Server 24.04 LTS to a USB stick (use Rufus or Balena Etcher).
2. Boot the OptiPlex from USB, install Ubuntu Server to the Micron SSD. Enable full-disk encryption (LUKS) at install time — this is the "holding your own encryption keys" step from §5. Save the passphrase in your password manager, not on the machine.
3. Set a hostname that means something — e.g. `hearth-01`.
4. Run updates, enable automatic security updates.

## Phase 2 — First services: private utility, no SaaS (~1–2 hrs)
The spec's product (§9) is "turnkey micro-servers running open-source local cloud stacks (Jellyfin, Docker containers)" for "private data storage." Start with the stack the spec names:
1. Install Docker + Docker Compose.
2. Jellyfin — media server, the first visible "private utility."
3. A photo/file backup target (Immich or Nextcloud) — this is the direct SaaS-cost avoidance from §5: every photo that lands on the node instead of a paid cloud plan is a dollar the cascade keeps.
4. Harden SSH: key-only auth, no root login, fail2ban.

## Phase 3 — Validation: the floor test (~30 min)
1. Unplug the Ethernet cable. Verify Jellyfin and file access still work on the local network. This is the §10 Step 1 acceptance test: "validating offline storage and private utility without ongoing SaaS costs."
2. Note idle and load power draw (a cheap Kill-A-Watt or the UPS readout). Step 2 of the plan ("The Pilot Circle") will need empirical power benchmarks — record them now so the data exists when you need it.
3. Set up a backup routine for the node itself: external USB drive, scheduled rsync. The ratchet only holds if the floor is backed up — 3 copies, 2 media, 1 off-site (a trusted person's node later).

## Expansion notes (later, not now)
- The empty 3.5" bay is the bulk-storage path when 256GB runs out — a large HDD for media/photos.
- The M.2 slot is a future cache/fast-boot option.

## Decisions (locked as working defaults — override anytime)
- **OS:** Ubuntu Server 24.04 LTS (broader Jellyfin/Docker docs, fewer surprises than Debian for a first build).
- **First service:** Immich for photos (lighter than Nextcloud; photos are the clearest win against paid cloud plans). Nextcloud later if you want the everything-server.
- **Install mode:** guided walkthrough with Luna, phase by phase, whenever you're ready to start.

*Drafted 2026-10-14 from your Master Architectural Specification (§5, §9, §10) and your eBay hardware. Nothing here commits money or changes anything — it's a checklist for when you're ready to start.*
