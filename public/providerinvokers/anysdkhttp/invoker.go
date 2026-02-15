package anysdkhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/httpelement"
	"github.com/stackql/any-sdk/pkg/logging"
	"github.com/stackql/any-sdk/pkg/response"

	sdk_internal_dto "github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/providerinvoker"
	"github.com/stackql/any-sdk/public/formulation"
)

type itemsDTO struct {
	items             interface{}
	ok                bool
	isNilPayload      bool
	singletonResponse map[string]interface{}
}

func (id *itemsDTO) GetItems() (interface{}, bool) {
	return id.items, id.items != nil
}

func (id *itemsDTO) GetSingltetonResponse() (map[string]interface{}, bool) {
	return id.singletonResponse, id.singletonResponse != nil
}

func (id *itemsDTO) IsOk() bool {
	return id.ok
}

func (id *itemsDTO) IsNilPayload() bool {
	return id.isNilPayload
}

func newItemisationResult(
	items interface{},
	ok bool,
	isNilPayload bool,
	singletonResponse map[string]interface{},
) providerinvoker.ItemisationResult {
	return &itemsDTO{
		items:             items,
		ok:                ok,
		isNilPayload:      isNilPayload,
		singletonResponse: singletonResponse,
	}
}

type InsertPreparator interface {
	ActionInsertPreparation(payload providerinvoker.ActionInsertPayload) providerinvoker.ActionInsertResult
}
type methodElider interface {
	IsElide(string, ...any) bool
}

type actionInsertResult struct {
	err                error
	isHousekeepingDone bool
}

func NewPayload(
	armouryGenerator formulation.ArmouryGenerator,
	provider anysdk.Provider,
	method anysdk.OperationStore,
	tableName string,
	authCtx *dto.AuthCtx,
	runtimeCtx dto.RuntimeCtx,
	outErrFile io.Writer,
	maxResultsElement sdk_internal_dto.HTTPElement,
	elider methodElider,
	nilOK bool,
	polyHandler PolyHandler,
	selectItemsKey string,
	insertPreparator InsertPreparator,
	skipResponse bool,
	isMutation bool,
	isAwait bool,
	defaultHTTPClient *http.Client,
	messageHandler providerinvoker.MessageHandler,
) any {
	return standardPayload{
		ArmouryGenerator:  armouryGenerator,
		Provider:          provider,
		Method:            method,
		TableName:         tableName,
		AuthCtx:           authCtx,
		RuntimeCtx:        runtimeCtx,
		OutErrFile:        outErrFile,
		MaxResultsElement: maxResultsElement,
		Elider:            elider,
		NilOK:             nilOK,
		PolyHandler:       polyHandler,
		SelectItemsKey:    selectItemsKey,
		InsertPreparator:  insertPreparator,
		SkipResponse:      skipResponse,
		IsMutation:        isMutation,
		IsAwait:           isAwait,
		DefaultHTTPClient: defaultHTTPClient,
		messageHandler:    messageHandler,
	}
}

// standardPayload is the (still-internal) call-shape for the any-sdk HTTP invoker.
// It mirrors the data StackQL currently passes to newHTTPAgnosticatePayload()/agnosticate().
type standardPayload struct {
	ArmouryGenerator  formulation.ArmouryGenerator
	Provider          anysdk.Provider
	Method            anysdk.OperationStore
	TableName         string
	AuthCtx           *dto.AuthCtx
	RuntimeCtx        dto.RuntimeCtx
	OutErrFile        io.Writer
	MaxResultsElement sdk_internal_dto.HTTPElement
	Elider            methodElider
	NilOK             bool
	PolyHandler       PolyHandler
	SelectItemsKey    string
	InsertPreparator  InsertPreparator
	SkipResponse      bool
	IsMutation        bool
	IsAwait           bool
	DefaultHTTPClient *http.Client
	messageHandler    providerinvoker.MessageHandler
}

type Invoker struct{}

func New() providerinvoker.Invoker { return &Invoker{} }

func (i *Invoker) Invoke(ctx context.Context, req providerinvoker.Request) (providerinvoker.Result, error) {
	p, ok := req.Payload.(standardPayload)
	if !ok {
		return providerinvoker.Result{}, fmt.Errorf("anysdkhttp: unexpected payload type %T", req.Payload)
	}
	// Ensure poly handler exists (mono previously created one outside; we keep that optional).
	poly := p.PolyHandler
	if poly == nil && p.messageHandler != nil {
		poly = NewStandardPolyHandler(p.messageHandler)
	}

	agPayload := newHTTPAgnosticatePayload(
		p.ArmouryGenerator,
		p.Provider,
		p.Method,
		p.TableName,
		p.AuthCtx,
		p.RuntimeCtx,
		p.OutErrFile,
		p.MaxResultsElement,
		p.Elider,
		p.NilOK,
		poly,
		p.SelectItemsKey,
		p.InsertPreparator,
		p.SkipResponse,
		p.IsMutation,
		p.IsAwait,
		p.DefaultHTTPClient,
	)

	processorResponse, err := agnosticate(agPayload)
	if err != nil {
		return providerinvoker.Result{}, err
	}

	var msgs []string
	if poly != nil {
		msgs = append(msgs, poly.GetMessages()...)
	}
	if processorResponse != nil {
		if sm := processorResponse.GetSuccessMessages(); len(sm) > 0 {
			msgs = append(msgs, sm...)
		}
	}

	if processorResponse == nil {
		return providerinvoker.Result{Body: nil, Messages: msgs}, nil
	}
	return providerinvoker.Result{Body: processorResponse.GetSingletonBody(), Messages: msgs}, nil
}

type AgnosticatePayload interface {
	GetArmoury() (anysdk.HTTPArmoury, error)
	GetProvider() anysdk.Provider
	GetMethod() anysdk.OperationStore
	GetTableName() string
	GetAuthContext() *dto.AuthCtx
	GetRuntimeCtx() dto.RuntimeCtx
	GetOutErrFile() io.Writer
	GetMaxResultsElement() sdk_internal_dto.HTTPElement
	GetElider() methodElider
	IsNilResponseAcceptable() bool
	GetPolyHandler() PolyHandler
	GetSelectItemsKey() string
	GetInsertPreparator() InsertPreparator
	IsSkipResponse() bool
	IsMutation() bool
	IsAwait() bool
	GetDefaultHTTPClient() *http.Client // testing purposes only
}

type httpAgnosticatePayload struct {
	armouryGenerator        formulation.ArmouryGenerator
	provider                anysdk.Provider
	method                  anysdk.OperationStore
	tableName               string
	authCtx                 *dto.AuthCtx
	rtCtx                   dto.RuntimeCtx
	outErrFile              io.Writer
	maxResultsElement       sdk_internal_dto.HTTPElement
	elider                  methodElider
	isNilResponseAcceptable bool
	polyHandler             PolyHandler
	selectItemsKey          string
	insertPreparator        InsertPreparator
	isSkipResponse          bool
	isMutation              bool
	isAwait                 bool
	defaultHTTPClient       *http.Client // testing purposes only
}

func newHTTPAgnosticatePayload(
	armouryGenerator formulation.ArmouryGenerator,
	provider anysdk.Provider,
	method anysdk.OperationStore,
	tableName string,
	authCtx *dto.AuthCtx,
	rtCtx dto.RuntimeCtx,
	outErrFile io.Writer,
	maxResultsElement sdk_internal_dto.HTTPElement,
	elider methodElider,
	isNilResponseAcceptable bool,
	polyHandler PolyHandler,
	selectItemsKey string,
	insertPreparator InsertPreparator,
	isSkipResponse bool,
	isMutation bool,
	isAwait bool,
	defaultHTTPClient *http.Client,
) AgnosticatePayload {
	return &httpAgnosticatePayload{
		armouryGenerator:        armouryGenerator,
		provider:                provider,
		method:                  method,
		tableName:               tableName,
		authCtx:                 authCtx,
		rtCtx:                   rtCtx,
		outErrFile:              outErrFile,
		maxResultsElement:       maxResultsElement,
		elider:                  elider,
		isNilResponseAcceptable: isNilResponseAcceptable,
		polyHandler:             polyHandler,
		selectItemsKey:          selectItemsKey,
		insertPreparator:        insertPreparator,
		isSkipResponse:          isSkipResponse,
		isMutation:              isMutation,
		isAwait:                 isAwait,
		defaultHTTPClient:       defaultHTTPClient,
	}
}

func (ap *httpAgnosticatePayload) GetPolyHandler() PolyHandler {
	return ap.polyHandler
}

func (ap *httpAgnosticatePayload) GetDefaultHTTPClient() *http.Client {
	return ap.defaultHTTPClient
}

func (ap *httpAgnosticatePayload) IsAwait() bool {
	return ap.isAwait
}

func (ap *httpAgnosticatePayload) GetInsertPreparator() InsertPreparator {
	return ap.insertPreparator
}

func (ap *httpAgnosticatePayload) GetSelectItemsKey() string {
	return ap.selectItemsKey
}

func (ap *httpAgnosticatePayload) GetArmoury() (anysdk.HTTPArmoury, error) {
	return ap.armouryGenerator.GetHTTPArmoury()
}

func (ap *httpAgnosticatePayload) GetProvider() anysdk.Provider {
	return ap.provider
}

func (ap *httpAgnosticatePayload) IsMutation() bool {
	return ap.isMutation
}

func (ap *httpAgnosticatePayload) IsSkipResponse() bool {
	return ap.isSkipResponse
}

func (ap *httpAgnosticatePayload) GetMethod() anysdk.OperationStore {
	return ap.method
}

func (ap *httpAgnosticatePayload) GetTableName() string {
	return ap.tableName
}

func (ap *httpAgnosticatePayload) GetAuthContext() *dto.AuthCtx {
	return ap.authCtx
}

func (ap *httpAgnosticatePayload) GetRuntimeCtx() dto.RuntimeCtx {
	return ap.rtCtx
}

func (ap *httpAgnosticatePayload) GetOutErrFile() io.Writer {
	return ap.outErrFile
}

func (ap *httpAgnosticatePayload) GetMaxResultsElement() sdk_internal_dto.HTTPElement {
	return ap.maxResultsElement
}

func (ap *httpAgnosticatePayload) GetElider() methodElider {
	return ap.elider
}

func (ap *httpAgnosticatePayload) IsNilResponseAcceptable() bool {
	return ap.isNilResponseAcceptable
}

type PolyHandler interface {
	LogHTTPResponseMap(target interface{})
	MessageHandler([]string)
	GetMessages() []string
}

type standardPolyHandler struct {
	messageHandler providerinvoker.MessageHandler
	messages       []string
}

func (sph *standardPolyHandler) LogHTTPResponseMap(target interface{}) {
	sph.messageHandler.LogHTTPResponseMap(target)
}

func (sph *standardPolyHandler) MessageHandler(messages []string) {
	sph.messages = append(sph.messages, messages...)
}

func (sph *standardPolyHandler) GetMessages() []string {
	return sph.messages
}

func NewStandardPolyHandler(messageHandler providerinvoker.MessageHandler) PolyHandler {
	return &standardPolyHandler{
		messageHandler: messageHandler,
		messages:       []string{},
	}
}

func agnosticate(
	agPayload AgnosticatePayload,
) (ProcessorResponse, error) {
	outErrFile := agPayload.GetOutErrFile()
	runtimeCtx := agPayload.GetRuntimeCtx()
	provider := agPayload.GetProvider()
	tableName := agPayload.GetTableName()
	authCtx := agPayload.GetAuthContext()
	method := agPayload.GetMethod()
	mr := agPayload.GetMaxResultsElement()
	elider := agPayload.GetElider()
	polyHandler := agPayload.GetPolyHandler()
	selectItemsKey := agPayload.GetSelectItemsKey()
	insertPreparator := agPayload.GetInsertPreparator()
	isSkipResponse := agPayload.IsNilResponseAcceptable()
	isMutation := agPayload.IsMutation()
	isAwait := agPayload.IsAwait()
	defaultHTTPClient := agPayload.GetDefaultHTTPClient()
	// TODO: TCC setup
	armoury, armouryErr := agPayload.GetArmoury()
	if armouryErr != nil {
		//nolint:errcheck // TODO: fix
		outErrFile.Write([]byte(
			fmt.Sprintf(
				"error assembling http aspects for resource '%s': %s\n",
				method.GetResource().GetID(),
				armouryErr.Error(),
			),
		),
		)
		return nil, armouryErr
	}
	if mr != nil {
		// TODO: infer param position and act accordingly
		ok := true
		if ok && runtimeCtx.HTTPMaxResults > 0 {
			passOverParams := armoury.GetRequestParams()
			for i, p := range passOverParams {
				param := p
				q := param.GetQuery()
				q.Set("maxResults", strconv.Itoa(runtimeCtx.HTTPMaxResults))
				param.SetRawQuery(q.Encode())
				passOverParams[i] = param
			}
			armoury.SetRequestParams(passOverParams)
		}
	}
	reqParams := armoury.GetRequestParams()
	logging.GetLogger().Infof("monoValentExecution.Execute() req param count = %d", len(reqParams))
	var processorResponse ProcessorResponse
	for _, rc := range reqParams {
		rq := rc
		processor := NewProcessor(
			NewProcessorPayload(
				rq,
				elider,
				provider,
				method,
				tableName,
				runtimeCtx,
				authCtx,
				outErrFile,
				polyHandler,
				selectItemsKey,
				insertPreparator,
				isSkipResponse,
				false,
				isAwait,
				false,
				isMutation,
				"",
				defaultHTTPClient,
			),
		)
		processorResponse = processor.Process()
		if processorResponse != nil && processorResponse.GetError() != nil {
			return processorResponse, processorResponse.GetError()
		}
	}
	return processorResponse, nil
}

type ProcessorPayload interface {
	GetArmouryParams() anysdk.HTTPArmouryParameters
	GetElider() methodElider
	GetProvider() anysdk.Provider
	GetMethod() anysdk.OperationStore
	GetTableName() string
	GetRuntimeCtx() dto.RuntimeCtx
	GetAuthCtx() *dto.AuthCtx
	GetOutErrFile() io.Writer
	GetPolyHandler() PolyHandler
	GetSelectItemsKey() string
	GetInsertPreparator() InsertPreparator
	IsSkipResponse() bool
	IsMutation() bool
	IsMaterialiseResponse() bool
	IsAwait() bool
	IsReverseRequired() bool
	GetVerb() string
	GetDefaultHTTPClient() *http.Client // testing purposes only
}

func NewProcessorPayload(
	armouryParams anysdk.HTTPArmouryParameters,
	elider methodElider,
	provider anysdk.Provider,
	method anysdk.OperationStore,
	tableName string,
	runtimeCtx dto.RuntimeCtx,
	authCtx *dto.AuthCtx,
	outErrFile io.Writer,
	polyHandler PolyHandler,
	selectItemsKey string,
	insertPreparator InsertPreparator,
	isSkipResponse bool,
	isMaterialiseResponse bool,
	isAwait bool,
	isReverseRequired bool,
	isMutation bool,
	verb string,
	defaultHTTPClient *http.Client,
) ProcessorPayload {
	return &standardProcessorPayload{
		armouryParams:         armouryParams,
		elider:                elider,
		provider:              provider,
		method:                method,
		tableName:             tableName,
		runtimeCtx:            runtimeCtx,
		authCtx:               authCtx,
		outErrFile:            outErrFile,
		polyHandler:           polyHandler,
		selectItemsKey:        selectItemsKey,
		insertPreparator:      insertPreparator,
		isSkipResponse:        isSkipResponse,
		isMaterialiseResponse: isMaterialiseResponse,
		isAwait:               isAwait,
		isReverseRequired:     isReverseRequired,
		isMutation:            isMutation,
		verb:                  verb,
		defaultHTTPClient:     defaultHTTPClient,
	}
}

type standardProcessorPayload struct {
	armouryParams         anysdk.HTTPArmouryParameters
	elider                methodElider
	provider              anysdk.Provider
	method                anysdk.OperationStore
	tableName             string
	runtimeCtx            dto.RuntimeCtx
	authCtx               *dto.AuthCtx
	outErrFile            io.Writer
	polyHandler           PolyHandler
	selectItemsKey        string
	insertPreparator      InsertPreparator
	isSkipResponse        bool
	isMaterialiseResponse bool
	isAwait               bool
	isReverseRequired     bool
	isMutation            bool
	verb                  string
	defaultHTTPClient     *http.Client // testing purposes only
}

func (pp *standardProcessorPayload) GetArmouryParams() anysdk.HTTPArmouryParameters {
	return pp.armouryParams
}

func (pp *standardProcessorPayload) IsSkipResponse() bool {
	return pp.isSkipResponse
}

func (pp *standardProcessorPayload) GetDefaultHTTPClient() *http.Client {
	return pp.defaultHTTPClient
}

func (pp *standardProcessorPayload) IsAwait() bool {
	return pp.isAwait
}

func (pp *standardProcessorPayload) IsMutation() bool {
	return pp.isMutation
}

func (pp *standardProcessorPayload) IsReverseRequired() bool {
	return pp.isReverseRequired
}

func (pp *standardProcessorPayload) IsMaterialiseResponse() bool {
	return pp.isMaterialiseResponse
}

func (pp *standardProcessorPayload) GetElider() methodElider {
	return pp.elider
}

func (pp *standardProcessorPayload) GetVerb() string {
	if pp.verb == "" {
		return "insert"
	}
	return pp.verb
}

func (pp *standardProcessorPayload) GetProvider() anysdk.Provider {
	return pp.provider
}

func (pp *standardProcessorPayload) GetMethod() anysdk.OperationStore {
	return pp.method
}

func (pp *standardProcessorPayload) GetTableName() string {
	return pp.tableName
}

func (pp *standardProcessorPayload) GetRuntimeCtx() dto.RuntimeCtx {
	return pp.runtimeCtx
}

func (pp *standardProcessorPayload) GetAuthCtx() *dto.AuthCtx {
	return pp.authCtx
}

func (pp *standardProcessorPayload) GetOutErrFile() io.Writer {
	return pp.outErrFile
}

func (pp *standardProcessorPayload) GetPolyHandler() PolyHandler {
	return pp.polyHandler
}

func (pp *standardProcessorPayload) GetSelectItemsKey() string {
	return pp.selectItemsKey
}

func (pp *standardProcessorPayload) GetInsertPreparator() InsertPreparator {
	return pp.insertPreparator
}

type ProcessorResponse interface {
	GetError() error
	GetSingletonBody() map[string]interface{}
	WithSuccessMessages([]string) ProcessorResponse
	GetSuccessMessages() []string
	AppendReversal(rev anysdk.HTTPPreparator)
	GetReversalStream() anysdk.HttpPreparatorStream
	IsFailed() bool
	GetFailedMessage() string
}

type httpProcessorResponse struct {
	body            map[string]interface{}
	err             error
	successMessages []string
	reversalStream  anysdk.HttpPreparatorStream
	isFailed        bool
	failedMessage   string
}

func (hpr *httpProcessorResponse) IsFailed() bool {
	return hpr.isFailed
}

func (hpr *httpProcessorResponse) GetFailedMessage() string {
	return hpr.failedMessage
}

func (hpr *httpProcessorResponse) WithSuccessMessages(messages []string) ProcessorResponse {
	hpr.successMessages = messages
	return hpr
}

//nolint:errcheck // acceptable for now
func (hpr *httpProcessorResponse) AppendReversal(rev anysdk.HTTPPreparator) {
	hpr.reversalStream.Write(rev)
}

func (hpr *httpProcessorResponse) GetReversalStream() anysdk.HttpPreparatorStream {
	return hpr.reversalStream
}

func (hpr *httpProcessorResponse) GetSuccessMessages() []string {
	return hpr.successMessages
}

func (hpr *httpProcessorResponse) GetError() error {
	return hpr.err
}

func (hpr *httpProcessorResponse) GetSingletonBody() map[string]interface{} {
	return hpr.body
}

func newHTTPProcessorResponse(
	body map[string]interface{},
	reversalStream anysdk.HttpPreparatorStream,
	isFailed bool,
	err error,
) ProcessorResponse {
	return &httpProcessorResponse{
		body:           body,
		err:            err,
		reversalStream: reversalStream,
		isFailed:       isFailed,
	}
}

type Processor interface {
	Process() ProcessorResponse
}

type standardProcessor struct {
	payload           ProcessorPayload
	defaultHTTPClient *http.Client
}

func NewProcessor(payload ProcessorPayload) Processor {
	return &standardProcessor{
		payload:           payload,
		defaultHTTPClient: payload.GetDefaultHTTPClient(),
	}
}

func inferNextPageRequestElement(provider anysdk.Provider, method anysdk.OperationStore) sdk_internal_dto.HTTPElement {
	st, ok := method.GetPaginationRequestTokenSemantic()
	if ok {
		if tp, err := sdk_internal_dto.ExtractHTTPElement(st.GetLocation()); err == nil {
			rv := sdk_internal_dto.NewHTTPElement(
				tp,
				st.GetKey(),
			)
			transformer, tErr := st.GetTransformer()
			if tErr == nil && transformer != nil {
				rv.SetTransformer(transformer)
			}
			return rv
		}
	}
	providerStr := provider.GetName()
	switch providerStr {
	case "github", "okta":
		return sdk_internal_dto.NewHTTPElement(
			sdk_internal_dto.RequestString,
			"",
		)
	default:
		return sdk_internal_dto.NewHTTPElement(
			sdk_internal_dto.QueryParam,
			"pageToken",
		)
	}
}

type PagingState interface {
	GetPageCount() int
	IsFinished() bool
	GetHTTPResponse() (*http.Response, error)
	GetAPIError() error
}

type httpPagingState struct {
	pageCount  int
	isFinished bool
	response   client.AnySdkResponse
	apiErr     error
}

func (hps *httpPagingState) GetPageCount() int {
	return hps.pageCount
}

func (hps *httpPagingState) IsFinished() bool {
	return hps.isFinished
}

func (hps *httpPagingState) GetHTTPResponse() (*http.Response, error) {
	if hps.response != nil {
		return hps.response.GetHttpResponse()
	}
	return nil, fmt.Errorf("nil http response in paging state")
}

func (hps *httpPagingState) GetAPIError() error {
	return hps.apiErr
}

func newPagingState(
	pageCount int,
	isFinished bool,
	response client.AnySdkResponse,
	apiErr error,
) PagingState {
	return &httpPagingState{
		pageCount:  pageCount,
		isFinished: isFinished,
		response:   response,
		apiErr:     apiErr,
	}
}

//nolint:funlen,bodyclose,gocognit,gocyclo,cyclop // acceptable for now
func (sp *standardProcessor) Process() ProcessorResponse {
	processorPayload := sp.payload
	armouryParams := processorPayload.GetArmouryParams()
	elider := processorPayload.GetElider()
	provider := processorPayload.GetProvider()
	method := processorPayload.GetMethod()
	tableName := processorPayload.GetTableName()
	runtimeCtx := processorPayload.GetRuntimeCtx()
	authCtx := processorPayload.GetAuthCtx()
	outErrFile := processorPayload.GetOutErrFile()
	polyHandler := processorPayload.GetPolyHandler()
	selectItemsKey := processorPayload.GetSelectItemsKey()
	insertPreparator := processorPayload.GetInsertPreparator()
	isSkipResponse := processorPayload.IsSkipResponse()
	isMutation := processorPayload.IsMutation()
	isMaterialiseResponse := processorPayload.IsMaterialiseResponse()
	isAwait := processorPayload.IsAwait()
	isReverseRequired := processorPayload.IsReverseRequired()
	verb := processorPayload.GetVerb()

	reversalStream := anysdk.NewHttpPreparatorStream()

	reqCtx := armouryParams
	paramsUsed, paramErr := reqCtx.ToFlatMap()
	if paramErr != nil {
		return newHTTPProcessorResponse(nil, reversalStream, false, paramErr)
	}
	reqEncoding := reqCtx.Encode()
	elideOk := elider.IsElide(reqEncoding)
	if elideOk {
		return newHTTPProcessorResponse(nil, reversalStream, false, nil)
	}
	// TODO: fix cloning ops
	cc := anysdk.NewAnySdkClientConfigurator(runtimeCtx, provider.GetName(), sp.defaultHTTPClient)
	response, apiErr := anysdk.CallFromSignature(
		cc,
		runtimeCtx,
		authCtx,
		authCtx.Type,
		false,
		outErrFile,
		provider,
		anysdk.NewAnySdkOpStoreDesignation(method),
		reqCtx.GetArgList(),
	)
	if response == nil {
		if apiErr != nil {
			return newHTTPProcessorResponse(nil, reversalStream, false, apiErr)
		}
		return newHTTPProcessorResponse(nil, reversalStream, false, fmt.Errorf("unacceptable nil response from HTTP call"))
	}
	//nolint:govet // ignore for now
	if isSkipResponse && response == nil {
		return newHTTPProcessorResponse(nil, reversalStream, false, nil)
	}
	httpResponse, httpResponseErr := response.GetHttpResponse()
	if httpResponse != nil && httpResponse.Body != nil {
		defer httpResponse.Body.Close()
	}
	if httpResponse != nil && httpResponse.StatusCode >= 400 && isMaterialiseResponse {
		generatedErr := fmt.Errorf("%s over HTTP error: %s", verb, httpResponse.Status)
		return newHTTPProcessorResponse(nil, reversalStream, true, generatedErr)
	}
	// TODO: refactor into package !!TECH_DEBT!!
	housekeepingDone := false
	nptRequest := inferNextPageRequestElement(provider, method)
	pageCount := 1
	for {
		if apiErr != nil {
			return newHTTPProcessorResponse(nil, reversalStream, false, apiErr)
		}
		if httpResponseErr != nil {
			return newHTTPProcessorResponse(nil, reversalStream, false, httpResponseErr)
		}
		// TODO: add async monitor here
		processed, resErr := method.ProcessResponse(httpResponse)
		if resErr != nil {
			if isSkipResponse && isMutation && httpResponse.StatusCode < 300 {
				return newHTTPProcessorResponse(
					nil, reversalStream, false, nil,
				).WithSuccessMessages([]string{"The operation was despatched successfully"})
			}
			//nolint:errcheck // TODO: fix
			outErrFile.Write(
				[]byte(fmt.Sprintf("error processing response: %s\n", resErr.Error())),
			)
			if processed == nil {
				return newHTTPProcessorResponse(nil, reversalStream, false, resErr)
			}
		}
		reversal, reversalExists := processed.GetReversal()
		if reversalExists {
			reversalAppendErr := reversalStream.Write(reversal)
			if reversalAppendErr != nil {
				return newHTTPProcessorResponse(nil, reversalStream, false, reversalAppendErr)
			}
		}
		if !reversalExists && isReverseRequired {
			return newHTTPProcessorResponse(nil, reversalStream, false, resErr)
		}
		res, respOk := processed.GetResponse()
		if !respOk {
			return newHTTPProcessorResponse(nil, reversalStream, false, fmt.Errorf("response is not a valid response"))
		}
		if res.HasError() {
			polyHandler.MessageHandler([]string{res.Error()})
			return newHTTPProcessorResponse(nil, reversalStream, false, nil)
		}
		polyHandler.LogHTTPResponseMap(res.GetProcessedBody())
		logging.GetLogger().Infoln(fmt.Sprintf("monoValentExecution.Execute() response = %v", res))

		if selectItemsKey == "" {
			selectItemsKey = method.GetSelectItemsKey()
		}

		itemisationResult := itemise(res.GetProcessedBody(), resErr, selectItemsKey)

		if itemisationResult.IsNilPayload() {
			break
		}

		singletonResponse, hasSingletonResponse := itemisationResult.GetSingltetonResponse()
		if isMaterialiseResponse {
			msgs := shallowGenerateSuccessMessagesFromHeirarchy(isAwait)
			//nolint:gomnd,mnd // acceptable for now
			if httpResponse.StatusCode < 300 {
				if hasSingletonResponse {
					return newHTTPProcessorResponse(singletonResponse, reversalStream, false, nil).WithSuccessMessages(msgs)
				}
				return newHTTPProcessorResponse(nil, reversalStream, false, nil).WithSuccessMessages(msgs)
			}
			return newHTTPProcessorResponse(nil, reversalStream, false, nil)
		}
		//nolint:gomnd,mnd // acceptable for now
		if httpResponse.StatusCode >= 300 {
			return newHTTPProcessorResponse(nil, reversalStream, false, nil)
		}

		insertPrepResult := insertPreparator.ActionInsertPreparation(
			newHTTPActionInsertPayload(
				itemisationResult,
				housekeepingDone,
				tableName,
				paramsUsed,
				reqEncoding,
			),
		)
		housekeepingDone = insertPrepResult.IsHousekeepingDone()
		insertPrepErr, hasInsertPrepErr := insertPrepResult.GetError()
		if !isAwait && isSkipResponse && isMutation && httpResponse.StatusCode < 300 {
			return newHTTPProcessorResponse(
				nil, reversalStream, false, nil,
			).WithSuccessMessages([]string{"The operation was despatched successfully"})
		}
		if hasInsertPrepErr {
			return newHTTPProcessorResponse(nil, reversalStream, false, insertPrepErr)
		}

		pageResult := page(
			res,
			method,
			provider,
			reqCtx,
			pageCount,
			runtimeCtx,
			authCtx,
			outErrFile,
			sp.defaultHTTPClient,
		)
		httpResponse, httpResponseErr = pageResult.GetHTTPResponse()
		// if httpResponse != nil && httpResponse.Body != nil {
		// 	defer httpResponse.Body.Close()
		// }
		if httpResponseErr != nil {
			if hasSingletonResponse { // TODO: fix this horrid hack
				return newHTTPProcessorResponse(singletonResponse, reversalStream, false, nil)
			}
			return newHTTPProcessorResponse(nil, reversalStream, false, nil)
			// return sdk_internal_dto.NewErroneousExecutorOutput(httpResponseErr)
		}

		if pageResult.IsFinished() {
			return newHTTPProcessorResponse(nil, reversalStream, false, nil)
		}

		pageCount = pageResult.GetPageCount()

		apiErr = pageResult.GetAPIError()
	}
	if reqCtx.GetRequest() != nil {
		q := reqCtx.GetRequest().URL.Query()
		q.Del(nptRequest.GetName())
		reqCtx.SetRawQuery(q.Encode())
	}
	return newHTTPProcessorResponse(nil, reversalStream, false, nil)
}

func shallowGenerateSuccessMessagesFromHeirarchy(isAwait bool) []string {
	baseSuccessString := "The operation completed successfully"
	if !isAwait {
		baseSuccessString = "The operation was despatched successfully"
	}
	successMsgs := []string{
		baseSuccessString,
	}
	return successMsgs
}

func newHTTPActionInsertPayload(
	itemisationResult providerinvoker.ItemisationResult,
	housekeepingDone bool,
	tableName string,
	paramsUsed map[string]interface{},
	reqEncoding string,
) providerinvoker.ActionInsertPayload {
	return &httpActionInsertPayload{
		itemisationResult: itemisationResult,
		housekeepingDone:  housekeepingDone,
		tableName:         tableName,
		paramsUsed:        paramsUsed,
		reqEncoding:       reqEncoding,
	}
}

func inferNextPageResponseElement(provider anysdk.Provider, method anysdk.OperationStore) sdk_internal_dto.HTTPElement {
	st, ok := method.GetPaginationResponseTokenSemantic()
	if ok {
		if tp, err := sdk_internal_dto.ExtractHTTPElement(st.GetLocation()); err == nil {
			rv := sdk_internal_dto.NewHTTPElement(
				tp,
				st.GetKey(),
			)
			transformer, tErr := st.GetTransformer()
			if tErr == nil && transformer != nil {
				rv.SetTransformer(transformer)
			}
			return rv
		}
	}
	providerStr := provider.GetName()
	switch providerStr {
	case "github", "okta":
		rv := sdk_internal_dto.NewHTTPElement(
			sdk_internal_dto.Header,
			"Link",
		)
		rv.SetTransformer(anysdk.DefaultLinkHeaderTransformer)
		return rv
	default:
		return sdk_internal_dto.NewHTTPElement(
			sdk_internal_dto.BodyAttribute,
			"nextPageToken",
		)
	}
}

func page(
	res response.Response,
	method anysdk.OperationStore,
	provider anysdk.Provider,
	reqCtx anysdk.HTTPArmouryParameters,
	pageCount int,
	rtCtx dto.RuntimeCtx,
	authCtx *dto.AuthCtx,
	outErrFile io.Writer,
	defaultHTTPClient *http.Client,
) PagingState {
	npt := inferNextPageResponseElement(provider, method)
	nptRequest := inferNextPageRequestElement(provider, method)
	if npt == nil || nptRequest == nil {
		return newPagingState(pageCount, true, nil, nil)
	}
	tk := extractNextPageToken(res, npt)
	if tk == "" || tk == "<nil>" || tk == "[]" || (rtCtx.HTTPPageLimit > 0 && pageCount >= rtCtx.HTTPPageLimit) {
		return newPagingState(pageCount, true, nil, nil)
	}
	pageCount++
	req, reqErr := reqCtx.SetNextPage(method, tk, nptRequest)
	if reqErr != nil {
		return newPagingState(pageCount, true, nil, reqErr)
	}
	cc := anysdk.NewAnySdkClientConfigurator(rtCtx, provider.GetName(), defaultHTTPClient)
	response, apiErr := anysdk.CallFromSignature(
		cc, rtCtx, authCtx, authCtx.Type, false, outErrFile, provider,
		anysdk.NewAnySdkOpStoreDesignation(method),
		anysdk.NewwHTTPAnySdkArgList(req), // TODO: abstract
	)
	return newPagingState(pageCount, false, response, apiErr)
}

type httpActionInsertPayload struct {
	itemisationResult providerinvoker.ItemisationResult
	housekeepingDone  bool
	tableName         string
	paramsUsed        map[string]interface{}
	reqEncoding       string
}

func (ap *httpActionInsertPayload) GetItemisationResult() providerinvoker.ItemisationResult {
	return ap.itemisationResult
}

func (ap *httpActionInsertPayload) IsHousekeepingDone() bool {
	return ap.housekeepingDone
}

func (ap *httpActionInsertPayload) GetTableName() string {
	return ap.tableName
}

func (ap *httpActionInsertPayload) GetParamsUsed() map[string]interface{} {
	return ap.paramsUsed
}

func (ap *httpActionInsertPayload) GetReqEncoding() string {
	return ap.reqEncoding
}

//nolint:nestif // apathy
func itemise(
	target interface{},
	resErr error,
	selectItemsKey string,
) providerinvoker.ItemisationResult {
	var items interface{}
	var ok bool
	var singletonResponse map[string]interface{}
	logging.GetLogger().Infoln(fmt.Sprintf("monoValentExecution.Execute() target = %v", target))
	switch pl := target.(type) {
	// add case for xml object,
	case map[string]interface{}:
		singletonResponse = pl
		if selectItemsKey != "" && selectItemsKey != "/*" {
			items, ok = pl[selectItemsKey]
			if !ok {
				if resErr != nil {
					items = []interface{}{}
					ok = true
				} else {
					items = []interface{}{
						pl,
					}
					ok = true
				}
			}
		} else {
			items = []interface{}{
				pl,
			}
			ok = true
		}
	case []interface{}:
		items = pl
		ok = true
	case []map[string]interface{}:
		items = pl
		ok = true
	case nil:
		return newItemisationResult(nil, false, true, singletonResponse)
	}
	return newItemisationResult(items, ok, false, singletonResponse)
}

func extractNextPageToken(res response.Response, tokenKey sdk_internal_dto.HTTPElement) string {
	//nolint:exhaustive // TODO: review
	switch tokenKey.GetType() {
	case sdk_internal_dto.BodyAttribute:
		return extractNextPageTokenFromBody(res, tokenKey)
	case sdk_internal_dto.Header:
		return extractNextPageTokenFromHeader(res, tokenKey)
	}
	return ""
}

//nolint:bodyclose // acceptable for now
func extractNextPageTokenFromHeader(res response.Response, tokenKey sdk_internal_dto.HTTPElement) string {
	r := res.GetHttpResponse()
	if r == nil {
		return ""
	}
	header := r.Header
	if tokenKey.IsTransformerPresent() {
		tf, err := tokenKey.Transformer(header)
		if err != nil {
			return ""
		}
		rv, ok := tf.(string)
		if !ok {
			return ""
		}
		return rv
	}
	vals := header.Values(tokenKey.GetName())
	if len(vals) == 1 {
		return vals[0]
	}
	return ""
}

func extractNextPageTokenFromBody(res response.Response, tokenKey sdk_internal_dto.HTTPElement) string {
	elem, err := httpelement.NewHTTPElement(tokenKey.GetName(), "body")
	if err == nil {
		rawVal, rawErr := res.ExtractElement(elem)
		if rawErr == nil {
			switch v := rawVal.(type) {
			case []interface{}:
				if len(v) == 1 {
					return fmt.Sprintf("%v", v[0])
				}
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	body := res.GetProcessedBody()
	switch target := body.(type) { //nolint:gocritic // TODO: review
	case map[string]interface{}:
		tokenName := tokenKey.GetName()
		nextPageToken, ok := target[tokenName]
		if !ok || nextPageToken == "" {
			logging.GetLogger().Infoln("breaking out")
			return ""
		}
		tk, ok := nextPageToken.(string)
		if !ok {
			logging.GetLogger().Infoln("breaking out")
			return ""
		}
		return tk
	}
	return ""
}
