# Metal GPU Passthrough via MoltenVK to qemu VM

Research document exploring the feasibility of passing Apple Metal GPU capabilities through to a qemu virtual machine for accelerated Whisper transcription.

## Executive Summary

**Verdict: Not feasible with current technology.**

Metal GPU passthrough to qemu VMs is not currently possible on macOS. The recommended approach is to run whisper-server directly on the Mac Mini host (outside the VM) and have the VM access it via network, which is already implemented in our architecture.

## Background

### The Problem

We want to run Whisper AI transcription with GPU acceleration inside a qemu VM on a Mac Mini. The Mac Mini has Apple Silicon (M-series) or discrete AMD GPU, using Apple's Metal API for GPU compute.

### Why GPU Matters for Whisper

| Mode | Processing Time (1hr audio) | Notes |
|------|----------------------------|-------|
| CPU (8 cores) | 10-30 minutes | Varies by model size |
| Metal GPU | 2-5 minutes | 3-6x faster |
| CUDA (NVIDIA) | 1-3 minutes | Fastest, not on Mac |

## Technology Analysis

### MoltenVK

MoltenVK is a Vulkan implementation that translates Vulkan API calls to Metal.

**What it does:**
- Provides Vulkan API on macOS/iOS
- Translates Vulkan to Metal at runtime
- Used by games (e.g., DXVK + Wine) and applications

**Limitations:**
- Runs on the HOST, not guest
- Cannot expose Metal to a VM guest OS
- No support for GPU passthrough to VMs

### qemu GPU Options on macOS

| Option | Description | Metal Support |
|--------|-------------|---------------|
| virtio-gpu | Paravirtualized GPU | No (software rendering) |
| virtio-gpu-gl | OpenGL passthrough | No (basic 3D only) |
| vmsvga | VMware SVGA | No (2D only) |
| Cocoa display | macOS native | No (display only) |

**Why no passthrough exists:**
1. Apple doesn't provide SR-IOV (Single Root I/O Virtualization) for GPUs
2. Metal is tightly coupled to macOS kernel extensions
3. No PCIe passthrough support in HVF (Hypervisor.framework)
4. Apple Silicon uses unified memory architecture with no discrete GPU to pass

### What About VT-d / IOMMU?

Intel VT-d and AMD IOMMU enable GPU passthrough on x86 hardware. However:

- **Intel Macs:** VT-d exists but macOS doesn't expose it to qemu/HVF
- **Apple Silicon:** No equivalent IOMMU exposed to userspace
- **Conclusion:** Even with theoretically capable hardware, Apple doesn't support this use case

## Alternative Approaches

### Option 1: Host-based whisper-server (Recommended)

**Current architecture - already implemented.**

```
┌─────────────────────────────────────────────────────┐
│              Mac Mini Host                          │
│                                                     │
│  ┌─────────────────┐    ┌─────────────────┐        │
│  │   qemu VM       │───▶│ whisper-server  │        │
│  │   (Backend)     │    │ (Metal GPU)     │        │
│  └─────────────────┘    └─────────────────┘        │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**Pros:**
- Works today, no research needed
- Full Metal GPU acceleration
- Simple networking (VM talks to host on 10.0.2.2)

**Cons:**
- whisper-server runs outside container/VM (less isolated)
- Must manage two processes separately

### Option 2: Remote GPU Server

Use a dedicated Linux machine with NVIDIA GPU.

```
┌─────────────────┐         ┌─────────────────┐
│   Mac Mini      │         │  GPU Server     │
│   qemu VM       │────────▶│  (NVIDIA CUDA)  │
│                 │ network │  whisper-server │
└─────────────────┘         └─────────────────┘
```

**Pros:**
- Best performance (CUDA)
- True isolation
- Scalable (multiple workers)

**Cons:**
- Requires additional hardware
- Network latency for large files
- More complex infrastructure

### Option 3: Cloud GPU (On-Demand)

Use cloud GPU instances for burst transcription.

**Services:**
- AWS EC2 g4dn instances (NVIDIA T4)
- Google Cloud GPU VMs
- RunPod / Vast.ai (cheaper spot instances)

**Pros:**
- No hardware investment
- Scale to zero when not in use
- Access to latest GPUs

**Cons:**
- Per-minute/hour costs
- Network transfer time for videos
- Dependency on external service

### Option 4: Apple Neural Engine (ANE)

Use Core ML with the Neural Engine instead of Metal GPU.

**Current state:**
- whisper.cpp has experimental Core ML support
- Requires model conversion to Core ML format
- ANE provides ~2-3x speedup over CPU

**Pros:**
- Lower power consumption
- Available on Apple Silicon

**Cons:**
- Limited model support
- Still requires running on host (same issue as Metal)
- Performance lower than Metal GPU

## Verdict

### Short Term

Continue with **Option 1** (host-based whisper-server). This is already implemented and provides full Metal GPU acceleration with minimal complexity.

### Medium Term

If demand increases, consider **Option 2** (remote GPU server) with NVIDIA hardware for best performance and true isolation.

### Long Term

Monitor Apple's virtualization developments. Apple has been improving Virtualization.framework, but GPU passthrough is unlikely in the near term given their security model.

## Technical Deep Dive

### Why MoltenVK Can't Help

MoltenVK sits at the wrong layer:

```
Application (whisper-server)
       │
       ▼
   Vulkan API
       │
       ▼
   MoltenVK (translation layer)
       │
       ▼
   Metal API
       │
       ▼
   Metal Driver (kernel)
       │
       ▼
   Apple GPU Hardware
```

For GPU passthrough, we'd need:

```
VM Guest Application
       │
       ▼
   Vulkan/Metal API (in guest)
       │
       ▼
   Virtual GPU Driver (guest ──▶ host)
       │
       ▼
   Metal API (host)
       │
       ▼
   Apple GPU Hardware
```

This "Virtual GPU Driver" doesn't exist for Metal. The closest concept is:
- **virgl** (Virtio 3D) - OpenGL only, no compute shaders
- **Venus** (Vulkan passthrough) - Linux host only

### whisper.cpp GPU Backend Status

| Backend | macOS Support | Notes |
|---------|---------------|-------|
| Metal | Yes | Default for Apple Silicon |
| CUDA | No | NVIDIA only |
| OpenCL | Limited | Deprecated on macOS |
| Vulkan | Via MoltenVK | Works but no VM passthrough |

## References

- [MoltenVK GitHub](https://github.com/KhronosGroup/MoltenVK)
- [whisper.cpp Metal support](https://github.com/ggerganov/whisper.cpp#metal-support)
- [QEMU macOS documentation](https://wiki.qemu.org/Hosts/Mac)
- [Apple Virtualization.framework](https://developer.apple.com/documentation/virtualization)
- [virgl project](https://virgil3d.github.io/)

## Conclusion

Metal GPU passthrough to qemu VMs is not feasible with current technology due to:
1. No Apple-provided SR-IOV or IOMMU support for GPUs
2. No virtual GPU driver for Metal
3. Metal is tightly integrated with macOS kernel

The recommended architecture (whisper-server on host, accessed via network) provides the same GPU acceleration benefit without the complexity of passthrough.

## See Also

- [../specs/deployment.md](deployment.md) - Current deployment architecture
- [../backend/README.md](../backend/README.md) - whisper-server configuration
