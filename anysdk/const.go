package anysdk

const (
	MethodDescription string = "description"
	MethodName        string = "MethodName"
	RequiredParams    string = "RequiredParams"
	SQLVerb           string = "SQLVerb"
)

const (
	ExtensionKeyAlwaysRequired string = "x-alwaysRequired"
	ExtensionKeyGraphQL        string = "x-stackQL-graphQL"
	ExtensionKeyConfig         string = "x-stackQL-config"
	ExtensionKeyProvider       string = "x-stackql-provider"
	ExtensionKeyResources      string = "x-stackQL-resources"
	ExtensionKeyStringOnly     string = "x-stackQL-stringOnly"
)

const (
	requestBodyKeyPrefix    string = "data"
	requestBodyKeyDelimiter string = "__"
	requestBodyBaseKey      string = requestBodyKeyPrefix + requestBodyKeyDelimiter
)

const (
	ViewKeyResourceLevelSelect string = "select"
)
