# Github Action Script.go

This little github action let's you write the rest of your deployment script in Go. Behind the scene, it creates a plugin out of your Go code and executes them.

example of using this Github Action

```yml
name: Example
on:
  push:
    branches:
      - "main"

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Setup Go 1.18
        uses: actions/setup-go@v2
        with:
          go-version: ^1.18

      - name: Checking out ...
        uses: actions/checkout@v2

      - name: Scripts
        uses: alinz/script.go@main
        with:
          workspace: ${{ github.workspace }} # <- this is important
          paths: .github/workflows/one,.github/workflows/two #<- the path to your go scripts
```

> make sure to pass your workspace folder. Also each go script must be placed inside a unique folder and must have a go.mod file. For a real example, [checkout this repo](https://github.com/alinz/examples-script.go)
