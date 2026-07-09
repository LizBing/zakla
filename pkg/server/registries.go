package server

import "github.com/zakla/mc-server/pkg/protocol"

// Registry is one synchronized registry sent during configuration via Registry Data.
type Registry struct {
	ID      string
	Entries []protocol.RegistryEntry
}

// DefaultRegistries returns the full vanilla synchronized registry set,
// generated from the 26.2 client.jar data pack (see vanilla_data.go).
func DefaultRegistries() []Registry {
	return vanillaSyncRegistries
}
