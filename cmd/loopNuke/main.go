package main

import (
	loopnuke "github.com/JeffNeff/loopNuke/pkg/adapter"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
)

func main() {
	pkgadapter.Main("loopnuke-adapter", loopnuke.EnvAccessorCtor, loopnuke.NewAdapter)
}
