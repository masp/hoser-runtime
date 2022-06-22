# hoser-runtime

The Hoser runtime is responsible for taking a Hoser JSON graph file and executing it on actual hardware. The current implementation
does it by spawning and supervising OS processes and connecting their stdio together similar to Unix pipes in shell works.

## Getting Started

```
make install
hoser -h
```

See hoser-py on how to run sample pipelines using `hoser`.