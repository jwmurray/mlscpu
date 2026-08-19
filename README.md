# mlscpu

`mlscpu` is a tool designed for collecting information about the CPU on macOS. This tool takes inspiration from [`lscpu`](https://man7.org/linux/man-pages/man1/lscpu.1.html) tool used in Linux.

## Motivation

Although there are various commands that can retrieve this information (as mentioned in [this Stack Exchange thread](https://apple.stackexchange.com/questions/352769/does-macos-have-a-command-to-retrieve-detailed-cpu-information-like-proc-cpuinf)), there is currently no concrete tool for this purpose.

## Usage

To build `mlscpu`, run the following command:

```bash
go build -o mlscpu
```

This will create an executable file named `mlscpu`. Copy this file to the `/usr/local/bin/` directory using the following command:

```bash
cp mlscpu /usr/local/bin/
```

You can now run `mlscpu`
```bash
[14:55][~] mlscpu
Architecture: x86_64
Byte Order: Little Endian
CPU(s): 4
On-line CPU(s): 4
Thread(s) per core: 2
Core(s) per socket: 2
Socket(s): 1
Vendor ID: GenuineIntel
CPU family: 6
CPU Model: 61
Model name: Intel(R) Core(TM) i5-5250U CPU @ 1.60GHz
Stepping: 4
CPU MHz: 1600
CPU max MHz: 1600
CPU min MHz: 1600
Hyper-Threading Technology: Enabled
L1d cache: 32K
L1i cache: 32K
L2 cache: 256K
L3 cache: 3072K
Flags: FPU VME DE PSE TSC MSR PAE MCE CX8 APIC SEP MTRR PGE MCA CMOV PAT 
       PSE36 CLFSH DS ACPI MMX FXSR SSE SSE2 SS HTT TM PBE SSE3 PCLMULQDQ DTES64 
       MON DSCPL VMX EST TM2 SSSE3 FMA CX16 TPR PDCM SSE4.1 SSE4.2 x2APIC MOVBE 
       POPCNT AES PCID XSAVE OSXSAVE SEGLIM64 TSCTMR AVX1.0 RDRAND F16C
[14:59][mlscpu] 
```

### Apple Silicon

Apple Silicon exposes CPU details through a different set of sysctls than Intel
Macs: the `machdep.cpu.*` keys carrying vendor, family, model, stepping and
feature flags do not exist, and neither do `hw.cpufrequency*` or
`hw.l3cachesize`. `mlscpu` detects the architecture and reports the fields the
machine actually has, rather than printing empty values for the rest.

Cores and caches are listed per performance level, since the clusters differ
from one another and the flat `hw.l1dcachesize` family of keys describes only
one of them.

```bash
[11:37][~] mlscpu
Architecture: arm64
Byte Order: Little Endian
CPU(s): 18
On-line CPU(s): 18
Thread(s) per core: 1
Core(s) per socket: 18
Socket(s): 1
Vendor ID: Apple
Model name: Apple M5 Max
CPU family: 0xf76c5b1a
CPU subfamily: 5
Core(s) per perf level: 6 (Super), 12 (Performance)
L1d cache: 128K (Super), 64K (Performance)
L1i cache: 192K (Super), 128K (Performance)
L2 cache: 16384K (Super), 8192K (Performance)
Cache line size: 128B
Flags: advsimd advsimd_hpfpcvt aes afp armv8_1_atomics armv8_2_fhm armv8_2_sha3
       armv8_2_sha512 armv8_3_compnum armv8_crc32 armv8_gpi bf16 bti crc32 cssc
       csv2 csv3 dit dotprod dpb dpb2 ebf16 ecv fcma fhm flagm flagm2
       floatingpoint fp16 fp_syncexceptions fpac fpaccombine frintts hbc i8mm
       jscvt lrcpc lrcpc2 lse lse2 mte mte2 mte4 mte_canonical_tags
       mte_no_address_tags mte_store_only neon neon_fp16 neon_hpfp pacimp pauth
       pauth2 pmull rdm rpres sb sha1 sha256 sha3 sha512 sme sme2 sme2p1
       sme_b16b16 sme_b16f32 sme_bi32i32 sme_f16f16 sme_f16f32 sme_f32f32
       sme_f64f64 sme_i16i32 sme_i16i64 sme_i8i32 sve_b16b16 ucnormal_mem wfxt
```

CPU frequency is not reported on Apple Silicon: no sysctl exposes it, and the
performance and efficiency clusters run at different clocks, so a single figure
would be misleading.
