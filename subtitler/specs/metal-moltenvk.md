# Metal GPU Passthrough via MoltenVK to qemu VM

Research document exploring building a custom qemu with MoltenVK support to expose Vulkan-compatible GPU to guest VMs on Apple Silicon.

## Executive Summary

**Goal:** Compile qemu with MoltenVK integration so guest VMs can access GPU compute via Vulkan API.

**Current Status:** Research completed January 2026. **Verdict: Not recommended for production use.**

**Summary:** While technical progress has been made (QEMU 9.2+ includes Venus, whisper.cpp has Vulkan support), macOS/MoltenVK integration remains blocked by memory mapping limitations. The current host-based whisper-server approach is more reliable.

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

## Research Findings (January 2026)

### QEMU Venus/Vulkan Support Status

**Good news:** Upstream QEMU 9.2.0+ now includes Venus support for Vulkan passthrough.

- Venus is available since QEMU 9.2.0 and Linux kernel 6.13
- Enable with: `-device virtio-gpu-gl,hostmem=8G,blob=true,venus=true`
- Venus supports Vulkan API versions up to 1.3
- Translation handled by virglrenderer 1.0.0+

**References:**
- [QEMU VirtIO-GPU Vulkan Support (Phoronix)](https://www.phoronix.com/news/VirtIO-GPU-Vulkan-QEMU)
- [Venus Documentation (Mesa)](https://docs.mesa3d.org/drivers/venus.html)
- [State of GFX Virtualization (Collabora, Jan 2025)](https://www.collabora.com/news-and-blog/blog/2025/01/15/the-state-of-gfx-virtualization-using-virglrenderer/)

### macOS/Apple Silicon Integration Status

**Bad news:** Venus + MoltenVK on macOS has significant blockers.

From [UTM Issue #4551](https://github.com/utmapp/UTM/issues/4551):

> Venus requires specific Vulkan memory features that MoltenVK cannot implement on Apple's platform. Venus assumes device memory can be exported as memory-mapped DMA buffers, requiring Linux kernel UDMA buffer support. This functionality lacks implementation across macOS and MoltenVK.

**Workaround attempts:**
- Custom QEMU builds with patches exist ([osy's gist](https://gist.github.com/osy/a8f705050eed1c8421ad1a0855a8faa9))
- Requires patching QEMU, virglrenderer, MoltenVK, and libepoxy
- Performance is 75-77% of native Metal ([Red Hat](https://developers.redhat.com/articles/2025/06/05/how-we-improved-ai-inference-macos-podman-containers))
- Patches not upstreamed; maintenance burden is high

**Alternative: gfxstream** (from Google's Android emulator) is being explored but requires significant work to adapt for vanilla Linux/QEMU.

### whisper.cpp Vulkan Support Status

**Good news:** whisper.cpp now has excellent Vulkan support.

- [PR #2302](https://github.com/ggml-org/whisper.cpp/pull/2302) added Vulkan as GPU backend
- Version 1.8.3 (Jan 2026) delivers 12x speedup on integrated GPUs
- Works with AMD, NVIDIA, and Intel GPUs on Linux/Windows
- Users report Vulkan is ~10x faster than CPU, comparable to CUDA
- [Discussion #2375](https://github.com/ggml-org/whisper.cpp/discussions/2375) confirms positive user experiences

**However:** On Linux guests in a macOS QEMU VM, we'd need the Venus stack working, which brings us back to the macOS blockers above.

## Conclusion and Recommendation

### Why This Approach Is Not Recommended

1. **macOS blockers:** MoltenVK cannot implement required memory mapping features for Venus
2. **Patch maintenance:** Custom builds require ongoing maintenance of 4+ projects
3. **Performance penalty:** Even when working, ~25% slower than native Metal
4. **Complexity:** High failure modes, hard to debug GPU issues in VMs

### Recommended Approach

**Continue with host-based whisper-server** (current implementation):
- Already working and deployed
- Native Metal performance
- Simple architecture: VM calls whisper-server on host via `10.0.2.2:8765`
- No custom builds or patches needed

### Future Possibilities

Monitor these developments:
1. **Upstream macOS Venus support** - If MoltenVK/Apple adds DMA buffer support
2. **gfxstream on macOS** - Alternative serialization approach being explored
3. **Apple Virtualization Framework** - May eventually expose GPU to guests

## Known Challenges

1. ~~**Venus is Linux-only**~~ **Confirmed:** Venus requires features macOS/MoltenVK cannot provide
2. ~~**virtio-gpu Vulkan**~~ **Confirmed:** Upstream in QEMU 9.2+, but macOS backend blocked
3. ~~**MoltenVK limitations**~~ **Confirmed:** Cannot implement DMA buffer export required by Venus
4. ~~**whisper.cpp Vulkan**~~ **Resolved:** Vulkan backend merged and works well on native Linux

## References

- [MoltenVK GitHub](https://github.com/KhronosGroup/MoltenVK)
- [Venus Documentation](https://docs.mesa3d.org/drivers/venus.html)
- [virglrenderer](https://virgil3d.github.io/)
- [qemu virtio-gpu](https://www.qemu.org/docs/master/system/devices/virtio-gpu.html)
- [whisper.cpp](https://github.com/ggerganov/whisper.cpp)
- [QEMU on Apple Silicon with Vulkan support (osy's gist)](https://gist.github.com/osy/a8f705050eed1c8421ad1a0855a8faa9)
- [UTM Venus+MoltenVK Issue #4551](https://github.com/utmapp/UTM/issues/4551)
- [whisper.cpp Vulkan PR #2302](https://github.com/ggml-org/whisper.cpp/pull/2302)
- [State of GFX Virtualization (Collabora, Jan 2025)](https://www.collabora.com/news-and-blog/blog/2025/01/15/the-state-of-gfx-virtualization-using-virglrenderer/)
- [GPU-Accelerated AI Inference in Linux Container on macOS (Red Hat)](https://developers.redhat.com/articles/2025/06/05/how-we-improved-ai-inference-macos-podman-containers)

## See Also

- [deployment.md](deployment.md) - Current deployment architecture (fallback approach)
