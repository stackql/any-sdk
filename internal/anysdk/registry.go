package anysdk

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stackql/any-sdk/pkg/compression"
	"github.com/stackql/stackql-provider-registry/signing/pqsig"
	"gopkg.in/yaml.v3"

	"github.com/stackql/any-sdk/pkg/semver"
)

const (
	defaultRegistryUrlString string = "https://raw.githubusercontent.com/stackql/stackql-provider-registry/intial-devel/providers"
	defaultSrcPrefix         string = "src"
	defaultDistPrefix        string = "dist"
	httpSchemeRegexpString   string = `(?i)^https?$`
	fileSchemeRegexpString   string = `(?i)^file$`
	remoteProviderListPath   string = `providers.yaml`
)

var (
	httpSchemeRegexp *regexp.Regexp = regexp.MustCompile(httpSchemeRegexpString)
	fileSchemeRegexp *regexp.Regexp = regexp.MustCompile(fileSchemeRegexpString)
)

type RegistryAPI interface {
	PullAndPersistProviderArchive(string, string) error
	PullProviderArchive(string, string) (io.ReadCloser, error)
	ListAllAvailableProviders() (map[string]ProviderDescription, error)
	ListAllProviderVersions(string) (map[string]ProviderDescription, error)
	ListLocallyAvailableProviders() map[string]ProviderDescription
	GetDocBytes(string) ([]byte, error)
	GetLatestAvailableVersion(string) (string, error)
	GetLatestPublishedVersion(string) (string, error)
	GetResourcesShallowFromProvider(Provider, string) (ResourceRegister, error)
	GetResourcesShallowFromProviderService(ProviderService) (ResourceRegister, error)
	GetResourcesShallowFromURL(ps ProviderService) (ResourceRegister, error)
	GetService(ps ProviderService) (Service, error)
	GetServiceFragment(ProviderService, string) (Service, error)
	GetServiceFromProviderService(ProviderService) (Service, error)
	GetServiceDocBytes(string) ([]byte, error)
	GetResourcesRegisterDocBytes(string) ([]byte, error)
	LoadProviderByName(string, string) (Provider, error)
	RemoveProviderVersion(string, string) error
	ClearProviderCache(string) error
	GetLocalDocRoot() string
	GetLocalDocTrunk() string
	GetLocalProviderDocPath(prov string, version string) string
}

type RegistryConfig struct {
	RegistryURL      string                `json:"url" yaml:"url"`
	SrcPrefix        *string               `json:"srcPrefix" yaml:"srcPrefix"`
	DistPrefix       *string               `json:"distPrefix" yaml:"distPrefix"`
	AllowSrcDownload bool                  `json:"allowSrcDownload" yaml:"allowSrcDownload"`
	LocalDocRoot     string                `json:"localDocRoot" yaml:"localDocRoot"`
	VerifyConfig     *pqsig.VerifierConfig `json:"verifyConfig" yaml:"verifyConfig"`
}

type Registry struct {
	allowSrcDownload bool
	regUrl           *url.URL
	srcUrl           *url.URL
	distUrl          *url.URL
	localDocRoot     string
	localSrcPrefix   string
	localDistPrefix  string
	transport        http.RoundTripper
	verifier         pqsig.Verifier
	nopVerifier      bool
}

func NewRegistry(registryCfg RegistryConfig, transport http.RoundTripper) (RegistryAPI, error) {
	return newRegistry(registryCfg, transport)
}

func newRegistry(registryCfg RegistryConfig, transport http.RoundTripper) (RegistryAPI, error) {
	registryUrl := registryCfg.RegistryURL
	if registryUrl == "" {
		registryUrl = defaultRegistryUrlString
	}
	srcUrlStr := registryUrl
	srcPrefix := ""
	if registryCfg.SrcPrefix == nil {
		srcPrefix = defaultSrcPrefix
	} else {
		srcPrefix = *registryCfg.SrcPrefix
	}
	if srcPrefix != "" {
		srcUrlStr = fmt.Sprintf("%s/%s", registryUrl, srcPrefix)
	}
	distUrlStr := registryUrl
	distPrefix := ""
	if registryCfg.DistPrefix == nil {
		distPrefix = defaultDistPrefix
	} else {
		distPrefix = *registryCfg.DistPrefix
	}
	if distPrefix != "" {
		distUrlStr = fmt.Sprintf("%s/%s", registryUrl, distPrefix)
	}
	regUrl, err := url.Parse(registryUrl)
	if err != nil {
		return nil, err
	}
	srcUrl, err := url.Parse(srcUrlStr)
	if err != nil {
		return nil, err
	}
	distUrl, err := url.Parse(distUrlStr)
	if err != nil {
		return nil, err
	}
	var ver pqsig.Verifier
	nopVerify := false
	var keys map[string]string
	if registryCfg.VerifyConfig != nil {
		nopVerify = registryCfg.VerifyConfig.NopVerify
		keys = registryCfg.VerifyConfig.PublicKeys
	}
	if !nopVerify {
		ver, err = pqsig.NewVerifier(keys)
		if err != nil {
			return nil, err
		}
	}
	rv := &Registry{
		allowSrcDownload: registryCfg.AllowSrcDownload,
		regUrl:           regUrl,
		srcUrl:           srcUrl,
		distUrl:          distUrl,
		localDocRoot:     registryCfg.LocalDocRoot,
		localSrcPrefix:   srcPrefix,
		localDistPrefix:  distPrefix,
		transport:        transport,
		verifier:         ver,
		nopVerifier:      nopVerify,
	}
	return rv, nil
}

func (r *Registry) GetLocalDocRoot() string {
	return r.getLocalDocRoot()
}

func (r *Registry) ListLocallyAvailableProviders() map[string]ProviderDescription {
	return r.listLocalProviders()
}

type ProviderDescription struct {
	_        struct{}
	Versions []string `json:"versions" yaml: "versions"`
}

func (pd ProviderDescription) GetLatestVersion() (string, error) {
	return semver.FindLatestStable(pd.Versions)
}

type ProvidersList struct {
	_         struct{}
	Providers map[string]ProviderDescription `json:"providers" yaml: "providers"`
}

func NewProvidersList() ProvidersList {
	return ProvidersList{
		Providers: make(map[string]ProviderDescription),
	}
}

func (pl ProvidersList) GetLatestList() (ProvidersList, error) {
	m := make(map[string]ProviderDescription)
	for k, v := range pl.Providers {
		latest, err := semver.FindLatestStable(v.Versions)
		if err != nil {
			return NewProvidersList(), err
		}
		m[k] = ProviderDescription{Versions: []string{latest}}
	}
	return ProvidersList{Providers: m}, nil
}

func (pl ProvidersList) GetSingleProviderList(prov string) ProvidersList {
	m := make(map[string]ProviderDescription)
	for k, v := range pl.Providers {
		if k == prov {
			m[k] = v
		}
	}
	return ProvidersList{Providers: m}
}

func (r *Registry) ListAllAvailableProviders() (map[string]ProviderDescription, error) {
	if r.isFile() {
		return nil, fmt.Errorf("'registry list' is meaningless in local mode")
	}
	regProvs := NewProvidersList()
	rc, err := r.getRemoteProviderList()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, regProvs)
	if err != nil {
		return nil, err
	}
	latest, err := regProvs.GetLatestList()
	if err != nil {
		return nil, err
	}
	return latest.Providers, nil
}

func (r *Registry) ListAllProviderVersions(prov string) (map[string]ProviderDescription, error) {
	return r.listAllProviderVersions(prov)
}

func (r *Registry) listAllProviderVersions(prov string) (map[string]ProviderDescription, error) {
	if r.isFile() {
		return nil, fmt.Errorf("'registry list' is meaningless in local mode")
	}
	regProvs := NewProvidersList()
	rc, err := r.getRemoteProviderList()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, regProvs)
	if err != nil {
		return nil, err
	}
	singleProvList := regProvs.GetSingleProviderList(prov)
	return singleProvList.Providers, nil
}

func (r *Registry) isHttp() bool {
	return httpSchemeRegexp.MatchString(r.regUrl.Scheme)
}

func (r *Registry) isFile() bool {
	return fileSchemeRegexp.MatchString(r.regUrl.Scheme)
}

func (r *Registry) isLocalFile() bool {
	return r.isFile() && strings.HasPrefix(r.regUrl.Path, "/")
}

func (r *Registry) GetDocBytes(docPath string) ([]byte, error) {
	return r.getVerifiedDocBytes(docPath)
}

func (r *Registry) getProviderDocBytes(prov string, version string) ([]byte, error) {
	prov = r.renameProvider(prov)
	return r.getVerifiedDocBytes(path.Join(prov, version, "provider.yaml"))
}

func (r *Registry) PullProviderArchive(prov string, version string) (io.ReadCloser, error) {
	return r.pullProviderArchive(prov, version)
}

func (r *Registry) pullProviderArchive(prov string, version string) (io.ReadCloser, error) {
	prov = r.renameProvider(prov)
	fp := path.Join(prov, fmt.Sprintf("%s.tgz", version))
	return r.pullArchive(fp)
}

func (r *Registry) PullAndPersistProviderArchive(prov string, version string) error {
	return r.pullAndPersistProviderArchive(prov, version)
}

func (r *Registry) pullAndPersistProviderArchive(prov string, version string) error {
	if r.localDocRoot == "" {
		return fmt.Errorf("cannot pull provider without local doc location")
	}
	rdr, err := r.pullProviderArchive(prov, version)
	if err != nil {
		return err
	}
	pr := r.renameProvider(prov)
	err = os.RemoveAll(path.Join(r.getLocalDocRoot(), pr, version))
	if err != nil {
		return err
	}
	return compression.DecompressToPath(rdr, path.Join(r.getLocalDocRoot(), pr))
}

func (r *Registry) LoadProviderByName(prov string, version string) (Provider, error) {
	b, err := r.getProviderDocBytes(prov, version)
	if err != nil {
		return nil, err
	}
	return LoadProviderDocFromBytes(b)
}

func (r *Registry) GetServiceDocBytes(url string) ([]byte, error) {
	return r.getVerifiedDocBytes(url)
}

func (r *Registry) GetResourcesRegisterDocBytes(url string) ([]byte, error) {
	return r.getVerifiedDocBytes(url)
}

func (r *Registry) GetService(ps ProviderService) (Service, error) {
	url := ps.getServiceRefRef()
	b, err := r.getVerifiedDocBytes(url)
	if err != nil {
		return nil, err
	}
	return LoadServiceDocFromBytes(ps, b)
}
func (r *Registry) GetResourcesShallowFromProvider(pr Provider, serviceKey string) (ResourceRegister, error) {
	return pr.getResourcesShallowWithRegistry(r, serviceKey)
}

func (r *Registry) GetResourcesShallowFromProviderService(pr ProviderService) (ResourceRegister, error) {
	return pr.getResourcesShallowWithRegistry(r)
}

func (r *Registry) GetResourcesShallowFromURL(ps ProviderService) (ResourceRegister, error) {
	url := ps.getResourcesRefRef()
	b, err := r.getVerifiedDocBytes(url)
	if err != nil {
		return nil, err
	}
	return loadResourcesShallow(ps, b)
}

func (r *Registry) GetServiceFromProviderService(ps ProviderService) (Service, error) {
	if ps.getServiceRefRef() == "" {
		return nil, fmt.Errorf("no service reachable for %s", ps.GetName())
	}
	return r.GetService(ps)
}

func (r *Registry) GetServiceFragment(ps ProviderService, resourceKey string) (Service, error) {

	if ps.getResourcesRefRef() == "" {
		if ps.getServiceRefRef() == "" {
			return nil, fmt.Errorf("no service or resources reachable for %s", ps.GetName())
		}
		return r.GetService(ps)
	}
	rr, err := r.GetResourcesShallowFromProviderService(ps)
	if err != nil {
		return nil, err
	}
	rsc, ok := rr.GetResource(resourceKey)
	if !ok {
		return nil, fmt.Errorf("cannot locate resource for key = '%s'", resourceKey)
	}
	sdRef := ps.getServiceDocRef(rr, rsc)
	if sdRef.Ref == "" {
		return nil, fmt.Errorf("no service doc available for resourceKey = '%s'", resourceKey)
	}
	if sdRef.Value != nil {
		return sdRef.Value, nil
	}
	sb, err := r.getVerifiedDocBytes(sdRef.Ref)
	if err != nil {
		return nil, err
	}
	svc, err := LoadServiceSubsetDocFromBytes(rr, resourceKey, sb)
	if err != nil {
		return nil, err
	}
	ps.setService(svc)
	return svc, nil
}

func (r *Registry) pullArchive(archivePath string) (io.ReadCloser, error) {
	return r.getUnVerifiedArchive(archivePath)
}

func (r *Registry) getRemoteDoc(docPath string) (io.ReadCloser, error) {
	cl := &http.Client{}
	if r.transport != nil {
		cl.Transport = r.transport
	}
	response, err := cl.Get(fmt.Sprintf("%s/%s", r.srcUrl.String(), docPath))
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return nil, fmt.Errorf("no response body from remote")
	}
	return response.Body, nil
}

func (r *Registry) getRemoteArchive(docPath string) (io.ReadCloser, error) {
	cl := &http.Client{}
	if r.transport != nil {
		cl.Transport = r.transport
	}
	response, err := cl.Get(fmt.Sprintf("%s/%s", r.distUrl.String(), docPath))
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return nil, fmt.Errorf("no response body from remote")
	}
	return response.Body, nil
}

func (r *Registry) getRemoteProviderList() (io.ReadCloser, error) {

	cl := &http.Client{}
	if r.transport != nil {
		cl.Transport = r.transport
	}
	response, err := cl.Get(fmt.Sprintf("%s/%s", r.distUrl.String(), remoteProviderListPath))
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return nil, fmt.Errorf("no response body from remote")
	}
	return response.Body, nil
}

func (r *Registry) GetLocalDocTrunk() string {
	return r.getLocalDocTrunk()
}

func (r *Registry) getLocalDocTrunk() string {
	return r.localDocRoot
}

func (r *Registry) getLocalDocRoot() string {
	switch r.localSrcPrefix {
	case "":
		return r.localDocRoot
	default:
		return path.Join(r.localDocRoot, r.localSrcPrefix)
	}
}

func (r *Registry) extractEmbeddedDocs() string {
	switch r.localSrcPrefix {
	case "":
		return r.localDocRoot
	default:
		return path.Join(r.localDocRoot, r.localSrcPrefix)
	}
}

type ProviderInfo struct {
	Name    string
	Version string
}

func (r *Registry) listLocalProviders() map[string]ProviderDescription {
	dr := r.getLocalDocRoot()
	switch dr {
	case "":
		return map[string]ProviderDescription{}
	default:
		provs, err := os.ReadDir(dr)
		if err != nil {
			return map[string]ProviderDescription{}
		}
		rv := make(map[string]ProviderDescription)
		for _, p := range provs {
			if p.IsDir() && !strings.HasPrefix(p.Name(), ".") {
				versions, err := os.ReadDir(path.Join(dr, p.Name()))
				var val ProviderDescription
				if err == nil {
					for _, v := range versions {
						if v.IsDir() && !strings.HasPrefix(v.Name(), ".") {
							val.Versions = append(val.Versions, v.Name())
						}
					}
				}
				rv[p.Name()] = val
			}
		}
		return rv
	}
}

func (r *Registry) getLocalArchiveRoot() string {
	switch r.localDistPrefix {
	case "":
		return r.localDocRoot
	default:
		return path.Join(r.localDocRoot, r.localDistPrefix)
	}
}

func (r *Registry) renameProvider(prov string) string {
	switch prov {
	case "google":
		prov = "googleapis.com"
	}
	return prov
}

func (r *Registry) GetLocalProviderDocPath(prov string, version string) string {
	return r.getLocalProviderDocPath(prov, version)
}

func (r *Registry) getLocalProviderDocPath(prov string, version string) string {
	return path.Join(r.getLocalProviderRootDirPath(prov, version), "provider.yaml")
}

func (r *Registry) getLocalProviderRootDirPath(prov string, version string) string {
	prov = r.renameProvider(prov)
	return path.Join(r.getLocalDocRoot(), prov, version)
}

func (r *Registry) getLocalDocPath(docPath string) string {
	return path.Join(r.getLocalDocRoot(), docPath)
}

func (r *Registry) getLocalArchivePath(docPath string) string {
	return path.Join(r.getLocalArchiveRoot(), docPath)
}

func (r *Registry) getLocalDoc(docPath string) (io.ReadCloser, error) {
	// localPath := r.getLocalDocPath(docPath)
	fi, err := os.Open(docPath)
	if fi != nil {
		defer fi.Close()
	} else {
		return nil, fmt.Errorf("nil file")
	}
	if err != nil {
		return nil, err
	}
	fb, readErr := io.ReadAll(fi)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read local file: '%s'", readErr.Error())
	}
	return io.NopCloser(bytes.NewReader(fb)), nil
}

func (r *Registry) getUnVerifiedArchive(docPath string) (io.ReadCloser, error) {
	if r.isLocalFile() {
		return os.Open(path.Join(r.distUrl.Path, docPath))
	}
	if r.localDocRoot != "" {
		localPath := r.getLocalArchivePath(docPath)
		lf, err := r.getLocalDoc(localPath)
		if err == nil {
			return lf, nil
		}
	}
	if r.isHttp() {
		cl := &http.Client{}
		if r.transport != nil {
			cl.Transport = r.transport
		}
		return r.getRemoteArchive(docPath)
	}
	return nil, fmt.Errorf("registry scheme '%s' currently not supported", r.regUrl.Scheme)
}

func (r *Registry) getDocReader(docPath string) (io.ReadCloser, error) {
	if r.isLocalFile() {
		return os.Open(path.Join(r.localSourcePath(), docPath))
	}
	if r.localDocRoot != "" {
		localPath := r.getLocalDocPath(docPath)
		lf, err := r.getLocalDoc(localPath)
		if err == nil {
			return lf, nil
		}
	}
	if r.isHttp() {
		if !r.allowSrcDownload {
			return nil, fmt.Errorf("download of individual docs disallowed; please attempt to pull provider docs")
		}
		response, err := r.getRemoteDoc(docPath)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, fmt.Errorf("no response body from remote")
		}
		return response, nil
	}
	return nil, fmt.Errorf("registry scheme '%s' currently not supported", r.regUrl.Scheme)
}

func (r *Registry) getVerifiedDocBytes(docPath string) ([]byte, error) {
	reader, err := r.getDocReader(docPath)
	if err != nil {
		return nil, fmt.Errorf("registry document %q: %w", docPath, err)
	}
	defer reader.Close()
	doc, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read registry document %q: %w", docPath, err)
	}
	if r.nopVerifier {
		return doc, nil
	}
	doc, err = r.verifier.Verify(doc)
	if err != nil {
		return nil, fmt.Errorf("verify registry document %q: %w", docPath, err)
	}
	return doc, nil
}

func (r *Registry) GetLatestAvailableVersion(providerName string) (string, error) {
	return r.getLatestAvailableVersion(providerName)
}

func (r *Registry) localSourcePath() string {
	localPath := r.srcUrl.Path
	if r.srcUrl.Host == "." {
		localPath = "." + localPath
	}
	if strings.HasPrefix(localPath, "/") && filepath.VolumeName(localPath[1:]) != "" {
		localPath = localPath[1:]
	}
	return localPath
}

func (r *Registry) getLatestAvailableVersion(providerName string) (string, error) {
	providerName = r.renameProvider(providerName)
	if r.isLocalFile() {
		deSlice, err := os.ReadDir(path.Join(r.localSourcePath(), providerName))
		if err != nil {
			return "", err
		}
		var deStrSlice []string
		for _, e := range deSlice {
			if e.IsDir() {
				deStrSlice = append(deStrSlice, e.Name())
			}
		}
		return semver.FindLatestStable(deStrSlice)
	}

	deSlice, err := os.ReadDir(path.Join(r.getLocalDocRoot(), providerName))
	if err != nil {
		return "", err
	}
	var deStrSlice []string
	for _, e := range deSlice {
		if e.IsDir() {
			deStrSlice = append(deStrSlice, e.Name())
		}
	}
	return semver.FindLatestStable(deStrSlice)
}

func (r *Registry) GetLatestPublishedVersion(providerName string) (string, error) {
	return r.getLatestPublishedVersion(providerName)
}

func (r *Registry) getLatestPublishedVersion(providerName string) (string, error) {
	p, err := r.listAllProviderVersions(providerName)
	if err != nil {
		return "", err
	}
	description, ok := p[providerName]
	if !ok {
		return "", fmt.Errorf("could not resolve provider '%s'", providerName)
	}
	latestVersion, err := semver.FindLatest(description.Versions)
	if err != nil {
		return "", err
	}
	return latestVersion, nil
}

// RemoveProviderVersion removes a specific version of a provider
// e.g) RemoveProviderVersion("okta", "v23.09.00169")
func (r *Registry) RemoveProviderVersion(providerId string, version string) error {
	providerPath := r.getLocalProviderRootDirPath(providerId, version)
	return os.RemoveAll(providerPath)
}

// ClearProviderCache clears the cache for a specific provider
// e.g) ClearProviderCache("aws")
func (r *Registry) ClearProviderCache(providerId string) error {
	cachePath := path.Join(r.getLocalDocRoot(), providerId)
	return os.RemoveAll(cachePath)
}
