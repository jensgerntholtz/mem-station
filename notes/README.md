# Notes

Jotting down some thoughts and ideas here as we solve problems.

Questions:
What is useful for a Memory Overclocker.
What is a typical workflow? - how could we reduce cognitive load which is usually expected when doing memory overclocking?

## System Info
What do we need to know about the system?
  - the memory modules that are installed
  - I would like to look up the notes on this Revision of the memory module as this determines certain characteristics of the module
  - what are the default timings and how do they compare to the JEDEC/XMP safe bounds?
  - what is the current memory configuration (timings, frequency, voltage)
  - what is the current system configuration (CPU, motherboard, etc.)

  - what is the theoretical maximum throughput given the current configuration?
  - if I were to change the timings, how would that affect the theoretical maximum throughput?

  - as I am chaning the timings I would like to know the safety margins. And have a clear indication of how far I am from the JEDEC/XMP safe bounds. (eg. highlight the timings that are outside the safe bounds, and show how far they are from the bounds)

## Benchmark
I want to have an accurate and deterministic benchmark that is representative of real-world performance as well as synthetic workloads.

I would like to see the results and how close we are to the theoretical maximum throughput.

Compare the results to previous runs and see if there is an improvement or regression.

I need results that I can compare against other users of similar systems / comparable skus. There needs to be validity in the results to prove that they haven't been tampered with.

## Burn-in
I need to verify the stability of my configuration.
A burn-in test should exceed the expected usage of the system in hopes to guarantee stability in real-world usage. It should be able to detect errors that may occur due to the changes in the memory configuration.

I need to configure the duration
I need to see the results of the burn-in test and see if there were any errors detected.
I want to be able to compare the results of the burn-in test to previous runs and see if there is an improvement or regression and which configuation was more stable - or any other variables that may have affected the stability (ambient temperature).

When the system encounters an issue there needs to be a visual queue and a way to shut down the system to prevent damage.

(check out stressapptest)

Other tooling:
1. Linpack (IntelBurnTest, LinX, y-cruncher)

    Focuses on floating-point and memory bandwidth.
    Brutally stressful, good for heat generation and memory subsystem errors.

2. Prime95 (Small FFTs, Large FFTs, Blend tests)

    Popular for CPU/core stability; "Blend" test pushes memory and IMC (Integrated Memory Controller) as well.
    Small FFT: Pure CPU, minimal memory.
    Large FFT/Blend: Memory and cache stress.

3. MemTest86/86+

    Pre-OS, boots from USB. Best for direct, low-level memory testing.
    Tests each addressable memory cell, catching errors missed by in-OS tools.

4. AIDA64 System Stability Test

    Allows stressing CPU, FPU, cache, RAM, and drives together or separately.
    Real-time monitoring/reporting.

5. OCCT (OverClock Checking Tool)

    Offers custom CPU, memory, power, and GPU stress patterns.
    Has built-in error checker, good for modern CPUs, AVX, and memory.

6. HCI MemTest

    Windows-based, run multiple instances in parallel (one for each logical thread).
    Great for catching memory errors in-OS quickly.

7. y-cruncher

    Multithreaded constant calculation (pi, e, √2, etc) using heavy RAM/CPU.
    Excellent for uncovering stability problems, especially with large memory sizes.

8. TestMem5 (TM5)

    Scriptable, advanced custom memory testing. Popular for DDR4/DDR5 overclockers.
    Config profiles for aggressive RAM timings.

9. Karhu RAM Test

    Paid, Windows, known for memory error detection speed and ease.
