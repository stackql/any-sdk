package discovery

import (
	"sort"

	"github.com/stackql/any-sdk/anysdk"
)

type Interrogator interface {
	GetProviderServices() ([]string, error)
}

type standardInterrogator struct {
	// add any necessary fields here
	providerRootDoc string
	registryRootDir string
}

func NewInterrogator(providerRootDoc, registryRootDir string) Interrogator {
	return &standardInterrogator{
		providerRootDoc: providerRootDoc,
		registryRootDir: registryRootDir,
	}
}

func (si *standardInterrogator) GetProviderServices() ([]string, error) {
	// Implement the logic to retrieve provider services
	// This is a placeholder implementation
	provider, fileErr := anysdk.LoadProviderDocFromFile(si.providerRootDoc)
	anysdk.OpenapiFileRoot = si.registryRootDir
	if fileErr != nil {
		return nil, fileErr
	}
	ps := provider.GetProviderServices()
	rv := make([]string, len(ps))
	i := 0
	for k := range ps {
		rv[i] = k
		i++
	}
	sort.Strings(rv)
	return rv, nil
}
