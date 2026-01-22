# Metal GPU Passthrough via MoltenVK to qemu VM

Research document exploring building a custom qemu with MoltenVK support to expose Vulkan-compatible GPU to guest VMs on Apple Silicon.

## Executive Summary

**Goal:** Compile qemu with MoltenVK integration so guest VMs can access GPU compute via Vulkan API.

**Current Status:** Requires research and testing. This document outlines the approach and test plan.

## Background

### The Problem

We want to run Whisper AI transcription with GPU acceleration inside a qemu VM on a Mac Mini M1. The guest VM should be able to use Vulkan/GPU compute APIs that get translated to Metal on the host.

### Why GPU Matters for Whisper

| Mode | Processing Time (1hr audio) | Notes |
|------|----------------------------|-------|
| CPU (8 cores) | 10-30 minutes | Varies by model size |
| Metal GPU | 2-5 minutes | 3-6x faster |
| CUDA (NVIDIA) | 1-3 minutes | Fastest, not on Mac |

## Approach: Custom qemu with MoltenVK

### What is MoltenVK?

[MoltenVK](https://github.com/KhronosGroup/MoltenVK) is a Vulkan implementation that translates Vulkan API calls to Metal at runtime.

**Key capabilities:**
- Provides full Vulkan 1.2 API on macOS
- Translates Vulkan → Metal at runtime
- Used by games (DXVK + Wine), applications, and potentially VMs
- Open source, actively maintained by Khronos Group

### The Idea

Build qemu with a GPU backend that:
1. Guest sees a Vulkan-compatible GPU (virtio-gpu with Vulkan)
2. Guest Vulkan calls are forwarded to host
3. Host uses MoltenVK to translate Vulkan → Metal
4. Metal executes on Apple GPU

```
┌─────────────────────────────────────────────────────────┐
│                    Guest VM (Linux)                      │
│  ┌─────────────────────────────────────────────────┐    │
│  │           whisper.cpp (Vulkan backend)           │    │
│  └──────────────────────┬──────────────────────────┘    │
│                         │ Vulkan API calls               │
│                         ▼                                │
│  ┌─────────────────────────────────────────────────┐    │
│  │         virtio-gpu (Vulkan passthrough)          │    │
│  └──────────────────────┬──────────────────────────┘    │
└─────────────────────────┼───────────────────────────────┘
                          │ virtio transport
┌─────────────────────────┼───────────────────────────────┐
│                    Host (macOS)                          │
│                         ▼                                │
│  ┌─────────────────────────────────────────────────┐    │
│  │              qemu virtio-gpu backend             │    │
│  └──────────────────────┬──────────────────────────┘    │
│                         │ Vulkan API calls               │
│                         ▼                                │
│  ┌─────────────────────────────────────────────────┐    │
│  │                   MoltenVK                        │    │
│  └──────────────────────┬──────────────────────────┘    │
│                         │ Metal API calls                │
│                         ▼                                │
│  ┌─────────────────────────────────────────────────┐    │
│  │                Apple M1 GPU                       │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### Related Projects to Study

1. **Venus** - Vulkan passthrough for virtio-gpu
   - Linux-only currently, but shows the architecture
   - https://docs.mesa3d.org/drivers/venus.html

2. **virglrenderer** - OpenGL passthrough for virtio-gpu
   - Similar concept but for OpenGL
   - Could be a template for Vulkan support

3. **crosvm** (Chrome OS VM) - Has Vulkan passthrough work
   - Google's VM for Chrome OS
   - May have macOS-relevant code

## Research Tasks

### Step 1: Clone and Study MoltenVK

```bash
cd ~/Github
git clone https://github.com/KhronosGroup/MoltenVK.git
cd MoltenVK

# Study the architecture
cat README.md
ls -la MoltenVK/
```

Questions to answer:
- How does MoltenVK expose the Vulkan API?
- Can it be linked as a library by other applications?
- What Metal features does it require?

### Step 2: Study qemu virtio-gpu Vulkan Support

```bash
# Clone qemu source
git clone https://gitlab.com/qemu-project/qemu.git
cd qemu

# Look for Vulkan/Venus support
grep -r "vulkan\|venus" hw/display/
grep -r "virtio-gpu" --include="*.c" hw/display/
```

Questions to answer:
- Does qemu have virtio-gpu Vulkan support?
- What's the status of Venus integration?
- What host backends are supported?

### Step 3: Check whisper.cpp Vulkan Support

```bash
cd ~/Github/whisper.cpp

# Check for Vulkan backend
grep -r "vulkan\|VULKAN" .
cat README.md | grep -i vulkan
```

Questions to answer:
- Does whisper.cpp support Vulkan compute?
- What GPU backends does it support? (Metal, CUDA, Vulkan, OpenCL)
- Can we add Vulkan support if missing?

## Test Plan

### Phase 1: Verify MoltenVK Works

**Prerequisites:**
- Xcode installed
- MoltenVK cloned

**Test:**
```bash
cd ~/Github/MoltenVK

# Build MoltenVK
./fetchDependencies --macos
make macos

# Run the Vulkan cube demo
./build/macos/Demos/vkcube
```

**Expected:** See a spinning Vulkan cube rendered via Metal.

### Phase 2: Build qemu with Vulkan Support

**Prerequisites:**
- qemu source
- MoltenVK built

**Test:** (Requires human assistance - commands TBD after research)
```bash
# Configure qemu with Vulkan support
cd ~/Github/qemu
./configure --enable-virtio-gpu-vulkan # flag TBD

# Build
make -j8
```

### Phase 3: Test in VM

**Test:**
```bash
# Start VM with Vulkan GPU
./qemu-system-aarch64 \
  -device virtio-gpu-vulkan # device name TBD
  ...

# In VM: run vulkaninfo
vulkaninfo

# In VM: run whisper with Vulkan
./whisper-cli --gpu vulkan ...
```

**Expected:** vulkaninfo shows GPU, whisper runs with GPU acceleration.

## Fallback: Host-based whisper-server

If qemu+MoltenVK doesn't work, continue with current architecture:

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

This is already implemented and working. VM accesses host via `10.0.2.2:8765`.

## Other Options

### Remote GPU Server

Use a dedicated Linux machine with NVIDIA GPU for best performance.

### Cloud GPU (On-Demand)

Use cloud GPU instances (AWS g4dn, RunPod) for burst transcription.

### Apple Neural Engine

whisper.cpp has experimental Core ML support for ANE acceleration.

## Known Challenges

1. **Venus is Linux-only** - The main Vulkan passthrough project targets Linux hosts
2. **virtio-gpu Vulkan** - May not be fully supported in upstream qemu for macOS
3. **MoltenVK limitations** - Some Vulkan features may not translate cleanly to Metal
4. **whisper.cpp Vulkan** - May not have a Vulkan backend (Metal is native)

## Next Steps

1. File a task to execute Phase 1 (verify MoltenVK works)
2. Research qemu Vulkan support status
3. Check if whisper.cpp can be modified to use Vulkan on Linux guest
4. Report findings and update this spec

## References

- [MoltenVK GitHub](https://github.com/KhronosGroup/MoltenVK)
- [Venus Documentation](https://docs.mesa3d.org/drivers/venus.html)
- [virglrenderer](https://virgil3d.github.io/)
- [qemu virtio-gpu](https://www.qemu.org/docs/master/system/devices/virtio-gpu.html)
- [whisper.cpp](https://github.com/ggerganov/whisper.cpp)

## See Also

- [deployment.md](deployment.md) - Current deployment architecture (fallback approach)
