//go:build !windows || !amd64

package icue

type unsupported struct{}

func Detected() bool            { return false }
func newBackend() backend       { return unsupported{} }
func (unsupported) read() State { return unavailable("unsupported") }
func (unsupported) close()      {}
