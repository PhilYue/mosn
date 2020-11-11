package proxywasm

import (
	"time"

	wasm "github.com/wasmerio/go-ext-wasm/wasmer"
	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
	"mosn.io/pkg/buffer"
)

type ContextBase interface {

	//
	// GeneralInterface
	//

	// Provides the status of the last call into the VM or out of the VM, similar to errno.
	// return the status code and a descriptive string.
	GetStatus() (statusCode int32, statusDescribe string, result WasmResult)

	// Return the current log level in the host
	GetLogLevel() (uint32, WasmResult)

	// Get the value of a property.  Some properties are proxy-independent (e.g. ["plugin_root_id"])
	// while others can be proxy-specific.
	GetProperty(key string) (string, WasmResult)

	// Set the value of a property
	SetProperty(key string, value string) WasmResult

	// Returns plugin configuration.
	GetConfiguration() string

	// Provides the current time in nanoseconds since the Unix epoch.
	GetCurrentTimeNanoseconds() (int, WasmResult)

	// Enables a periodic timer with the given period or sets the period of an existing timer. Note:
	// 		the timer is associated with the Root Context of whatever Context this call was made on.
	// @param period is the period of the periodic timer in milliseconds.  If the period is 0 the
	// 		timer is reset/deleted and will not call onTick.
	// @param timer is a pointer to the timer.  If the target of timer is
	// 		zero, a new timer will be allocated its token will be set.  If the target is non-zero, then
	// 		that timer will have the new period (or be reset/deleted if period is zero).
	SetTimerPeriod(period int64, timer uint32) WasmResult

	//
	// SharedDataInterface
	//
	// SharedDataInterface is for sharing data between VMs. In general the VMs may be on different
	// threads. Keys can have any format, but good practice would use reverse DNS and namespacing
	// prefixes to avoid conflicts.

	// Get proxy-wide key-value data shared between VMs.
	// @param key is a proxy-wide key mapping to the shared data value.
	// @param cas is a number which will be incremented when a data value has been changed.
	// @param data is a location to store the returned stored 'value' and the corresponding 'cas'
	// 		  compare-and-swap value which can be used with setSharedData for safe concurrent updates.
	GetSharedData(key string) (value string, cas uint32, result WasmResult)

	// Set a key-value data shared between VMs.
	// @param key is a proxy-wide key mapping to the shared data value.
	// @param cas is a compare-and-swap value. If it is zero it is ignored, otherwise it must match
	// 		the cas associated with the value.
	// @param data is a location to store the returned value.
	SetSharedData(key string, value string, cas uint32) WasmResult

	//
	// SharedQueueInterface
	//

	// Register a proxy-wide queue, return a token corresponding to the queue.
	RegisterSharedQueue(queueName string) (uint32, WasmResult)

	// Get the token for a queue, return the token corresponding to the queue.
	LookupSharedQueue(queueName string) (uint32, WasmResult)

	// Dequeue a message from a shared queue
	DequeueSharedQueue(queueToken uint32, data string) WasmResult

	// Enqueue a message on a shared queue
	EnqueueSharedQueue(queueToken uint32, data string) WasmResult

	//
	// MetricsInterface
	//

	// Define a metric (Stat)
	DefineMetric(metricType MetricType, name string) (uint32, WasmResult)

	// Increment a metric
	IncrementMetric(metricId uint32, offset int64) WasmResult

	// Record a metric
	RecordMetric(metricId uint32, value uint64) WasmResult

	// Get the current value of a metric
	GetMetric(metricId uint32) (uint64, WasmResult)

	//
	// Buffer/HeaderMap
	//
	GetBuffer(bufferType BufferType) ([]byte, WasmResult)
	SetBuffer(bufferType BufferType, buf []byte) WasmResult
	GetHeaderMap(mapType MapType) (api.HeaderMap, WasmResult)
	GetHeaderMapValue(mapType MapType, key string) (string, WasmResult)
	SetHeaderMapValue(mapType MapType, key string, value string) WasmResult
	AddHeaderMapValue(mapType MapType, key string, value string) WasmResult
	DelHeaderMapValue(mapType MapType, key string) WasmResult
}

type ProxyWasmInstance interface {
	GetMemory() []byte
	ProxyWasmExports
}

type ProxyWasmExports interface {
	_start() error
	malloc(size int32) (int32, error)

	proxy_on_context_create(contextId int32, parentContextId int32) error
	proxy_on_done(contextId int32) (int32, error)
	proxy_on_log(contextId int32) error
	proxy_on_delete(contextId int32) error

	proxy_on_vm_start(rootContextId int32, configurationSize int32) (int32, error)
	proxy_on_configure(rootContextId int32, configurationSize int32) (int32, error)

	proxy_on_tick(rootContextId int32) error

	proxy_on_new_connection(contextId int32) error
	proxy_on_downstream_data(contextId int32, dataLength int32, endOfStream int32) (int32, error)
	proxy_on_downstream_connection_close(contextId int32, closeType int32) error
	proxy_on_upstream_data(contextId int32, dataLength int32, endOfStream int32) (int32, error)
	proxy_on_upstream_connection_close(contextId int32, closeType int32) error

	proxy_on_request_headers(contextId int32, headers int32, endOfStream int32) (int32, error)
	proxy_on_request_body(contextId int32, bodyBufferLength int32, endOfStream int32) (int32, error)
	proxy_on_request_trailers(contextId int32, trailers int32) (int32, error)
	proxy_on_request_metadata(contextId int32, nElements int32) (int32, error)
	proxy_on_response_headers(contextId int32, headers int32, endOfStream int32) (int32, error)
	proxy_on_response_body(contextId int32, bodyBufferLength int32, endOfStream int32) (int32, error)
	proxy_on_response_trailers(contextId int32, trailers int32) (int32, error)
	proxy_on_response_metadata(contextId int32, nElements int32) (int32, error)

	proxy_on_http_call_response(contextId int32, token int32, headers int32, bodySize int32, trailers int32) error

	proxy_on_grpc_receive_initial_metadata(contextId int32, token int32, headers int32) error
	proxy_on_grpc_trailing_metadata(contextId int32, token int32, trailers int32) error
	proxy_on_grpc_receive(contextId int32, token int32, responseSize int32) error
	proxy_on_grpc_close(contextId int32, token int32, statusCode int32) error

	proxy_on_queue_ready(rootContextId int32, token int32) error

	proxy_validate_configuration(rootContextId int32, configurationSize int32) (int32, error)
	proxy_on_foreign_function(rootContextId int32, functionId int32, dataSize int32) error
}

type rootContext struct {
	config *StreamProxyWasmConfig

	//vmConfig     string
	//pluginConfig string
	//
	//contextId    uint32

	wasmCode      []byte
	wasmModule    wasm.Module
	wasiVersion   wasm.WasiVersion
	wasmImportObj *wasm.ImportObject

	propertyMap map[string]string

	metrics *metricsManager
}

func (ctx *rootContext) GetVmConfiguration() []byte {
	return nil
}

func (ctx *rootContext) GetPluginConfiguration() []byte {
	return nil
}

func (ctx *rootContext) GetProperty(key string) (string, WasmResult) {
	if ctx.propertyMap == nil {
		return "", WasmResultInternalFailure
	}
	if value, ok := ctx.propertyMap[key]; !ok {
		return "", WasmResultNotFound
	} else {
		return value, WasmResultOk
	}
}

func (ctx *rootContext) SetProperty(key string, value string) WasmResult {
	if ctx.propertyMap == nil {
		return WasmResultInternalFailure
	}
	ctx.propertyMap[key] = value
	return WasmResultOk
}

func (ctx *rootContext) DefineMetric(metricType MetricType, name string) (uint32, WasmResult) {
	switch metricType {
	case MetricTypeCounter:
		return ctx.metrics.newCounter(name), WasmResultOk
	case MetricTypeGauge:
		return ctx.metrics.newGauge(name), WasmResultOk
	case MetricTypeHistogram:
		return ctx.metrics.newHistogram(name), WasmResultOk
	}
	return 0, WasmResultBadArgument
}

func (ctx *rootContext) IncrementMetric(metricId uint32, offset int64) WasmResult {
	ctx.metrics.incrementMetric(metricId, offset)
	return WasmResultOk
}

func (ctx *rootContext) RecordMetric(metricId uint32, value uint64) WasmResult {
	ctx.metrics.recordMetric(metricId, int64(value))
	return WasmResultOk
}

func (ctx *rootContext) GetMetric(metricId uint32) (uint64, WasmResult) {
	value := ctx.metrics.getMetric(metricId)
	return uint64(value), WasmResultOk
}

type wasmContext struct {
	rootContext *rootContext
	contextId   int32
	filter      *streamProxyWasmFilter
	
	ProxyWasmInstance
}

func (wasm *wasmContext) SetTimerPeriod(period int64, timer uint32) WasmResult {
	log.DefaultLogger.Errorf("wasmContext.SetTimerPeriod() unimplemented")
	return WasmResultUnimplemented
}

func (wasm *wasmContext) GetCurrentTimeNanoseconds() (int, WasmResult) {
	return time.Now().Nanosecond(), WasmResultOk
}

func (wasm *wasmContext) DefineMetric(metricType MetricType, name string) (uint32, WasmResult) {
	return wasm.rootContext.DefineMetric(metricType, name)
}

func (wasm *wasmContext) IncrementMetric(metricId uint32, offset int64) WasmResult {
	return wasm.rootContext.IncrementMetric(metricId, offset)
}

func (wasm *wasmContext) RecordMetric(metricId uint32, value uint64) WasmResult {
	return wasm.rootContext.RecordMetric(metricId, value)
}

func (wasm *wasmContext) GetMetric(metricId uint32) (uint64, WasmResult) {
	return wasm.rootContext.GetMetric(metricId)
}

func (wasm *wasmContext) RegisterSharedQueue(queueName string) (uint32, WasmResult) {
	log.DefaultLogger.Errorf("instanceContext.RegisterSharedQueue() unimplemented")
	return 0, WasmResultUnimplemented
}

func (wasm *wasmContext) LookupSharedQueue(queueName string) (uint32, WasmResult) {
	log.DefaultLogger.Errorf("instanceContext.LookupSharedQueue() unimplemented")
	return 0, WasmResultUnimplemented
}

func (wasm *wasmContext) DequeueSharedQueue(queueToken uint32, data string) WasmResult {
	log.DefaultLogger.Errorf("instanceContext.DequeueSharedQueue() unimplemented")
	return WasmResultUnimplemented
}

func (wasm *wasmContext) EnqueueSharedQueue(queueToken uint32, data string) WasmResult {
	log.DefaultLogger.Errorf("instanceContext.EnqueueSharedQueue() unimplemented")
	return WasmResultUnimplemented
}

func (wasm *wasmContext) GetSharedData(key string) (value string, cas uint32, result WasmResult) {
	log.DefaultLogger.Errorf("instanceContext.GetSharedData() unimplemented")
	return "", 0, WasmResultUnimplemented
}

func (wasm *wasmContext) SetSharedData(key string, value string, cas uint32) WasmResult {
	log.DefaultLogger.Errorf("instanceContext.SetSharedData() unimplemented")
	return WasmResultUnimplemented
}

func (wasm *wasmContext) GetProperty(key string) (string, WasmResult) {
	return wasm.rootContext.GetProperty(key)
}

func (wasm *wasmContext) SetProperty(key string, value string) WasmResult {
	return wasm.rootContext.SetProperty(key, value)
}

func (wasm *wasmContext) GetBuffer(bufferType BufferType) ([]byte, WasmResult) {
	switch bufferType {
	case BufferTypeHttpRequestBody:
		return wasm.filter.rhandler.GetRequestData().Bytes(), WasmResultOk
	case BufferTypeHttpResponseBody:
		return wasm.filter.shandler.GetResponseData().Bytes(), WasmResultOk
	case BufferTypeVmConfiguration:
		return wasm.rootContext.GetVmConfiguration(), WasmResultOk
	case BufferTypePluginConfiguration:
		return wasm.rootContext.GetPluginConfiguration(), WasmResultOk
	default:
		return nil, WasmResultBadArgument
	}
}

func (wasm *wasmContext) SetBuffer(bufferType BufferType, buf []byte) WasmResult {
	switch bufferType {
	case BufferTypeHttpRequestBody:
		wasm.filter.rhandler.SetRequestData(buffer.NewIoBufferBytes(buf))
		return WasmResultOk
	case BufferTypeHttpResponseBody:
		wasm.filter.shandler.SetResponseData(buffer.NewIoBufferBytes(buf))
		return WasmResultOk
	default:
		return WasmResultBadArgument
	}
}

func (wasm *wasmContext) GetHeaderMap(mapType MapType) (api.HeaderMap, WasmResult) {
	switch mapType {
	case MapTypeHttpRequestHeaders:
		return wasm.filter.rhandler.GetRequestHeaders(), WasmResultOk
	case MapTypeHttpResponseHeaders:
		return wasm.filter.shandler.GetResponseHeaders(), WasmResultOk
	default:
		return nil, WasmResultBadArgument
	}
}

func (wasm *wasmContext) GetHeaderMapValue(mapType MapType, key string) (value string, result WasmResult) {
	var header api.HeaderMap
	switch mapType {
	case MapTypeHttpRequestHeaders:
		header = wasm.filter.rhandler.GetRequestHeaders()
		value, _ = header.Get(key)
	case MapTypeHttpResponseHeaders:
		header = wasm.filter.shandler.GetResponseHeaders()
		value, _ = header.Get(key)
	default:
		value = ""
		return value, WasmResultBadArgument
	}
	return value, WasmResultOk
}

func (wasm *wasmContext) SetHeaderMapValue(mapType MapType, key string, value string) WasmResult {
	var header api.HeaderMap
	switch mapType {
	case MapTypeHttpRequestHeaders:
		header = wasm.filter.rhandler.GetRequestHeaders()
		header.Set(key, value)
		return WasmResultOk
	case MapTypeHttpResponseHeaders:
		header = wasm.filter.shandler.GetResponseHeaders()
		header.Set(key, value)
		return WasmResultOk
	default:
		return WasmResultBadArgument
	}
}

func (wasm *wasmContext) AddHeaderMapValue(mapType MapType, key string, value string) WasmResult {
	var header api.HeaderMap
	switch mapType {
	case MapTypeHttpRequestHeaders:
		header = wasm.filter.rhandler.GetRequestHeaders()
		header.Add(key, value)
		return WasmResultOk
	case MapTypeHttpResponseHeaders:
		header = wasm.filter.shandler.GetResponseHeaders()
		header.Add(key, value)
		return WasmResultOk
	default:
		return WasmResultBadArgument
	}
}

func (wasm *wasmContext) DelHeaderMapValue(mapType MapType, key string) WasmResult {
	var header api.HeaderMap
	switch mapType {
	case MapTypeHttpRequestHeaders:
		header = wasm.filter.rhandler.GetRequestHeaders()
		header.Del(key)
		return WasmResultOk
	case MapTypeHttpResponseHeaders:
		header = wasm.filter.shandler.GetResponseHeaders()
		header.Del(key)
		return WasmResultOk
	default:
		return WasmResultBadArgument
	}
}
