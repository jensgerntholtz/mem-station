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