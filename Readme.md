# dockrun

[![GoDoc](https://godoc.org/matthewmueller/go-dockrun?status.svg)](https://godoc.org/github.com/matthewmueller/go-dockrun)

Simple container testing

## Install

```
go get github.com/matthewmueller/dockrun
```

## Example

```go
client, err := dockrun.New()
if err != nil {
  log.Fatal(err)
}

container, err := client.
  Container("yukinying/chrome-headless:latest", "chromium").
  Expose("9222:9222").
  Run(ctx)
if err != nil {
  log.Fatal(err)
}
defer container.Kill()

err = container.Wait(ctx, "http://localhost:9222")
if err != nil {
  log.Fatal(err)
}
```

## License 

MIT