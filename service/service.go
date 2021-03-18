package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-msvc/msf/logger"
	"github.com/go-msvc/msf/mux"
)

var log = logger.New("msf").New("service")

type IService interface {
	Handle(name string, handler interface{} /*checked at runtime: HandlerFunc*/) IService
	HandleMux(name string, mux mux.IMux) IService
	Run() error
	MustRun()
}

func NewService(name string) IService {
	return &service{
		name: name,
		mux:  mux.New(nil),
	}
}

type service struct {
	name string
	mux  mux.IMux
}

type handler struct {
	// //http handler
	// httpHandlerFunc http.HandlerFunc

	//custom request handler
	fncValue reflect.Value
	reqType  reflect.Type
}

func (s *service) Handle(name string, fnc interface{} /*HandlerFunc*/) IService {
	if fnc == nil {
		panic(fmt.Errorf("service(%s).handler(%s)=nil", s.name, name))
	}

	fncType := reflect.TypeOf(fnc)
	if fncType.Kind() != reflect.Func {
		panic(fmt.Errorf("service(%s).handler(%s)=%T is not a func", s.name, name, fnc))
	}

	s.mux.Add(name, handler{
		fncValue: reflect.ValueOf(fnc),
		reqType:  reflect.TypeOf(fnc).In(1),
	})
	return s
}

func (s *service) HandleMux(name string, mux mux.IMux) IService {
	s.mux.Add(name, mux)
	return s
}

func (s *service) Run() error {
	//todo: load config to start correct type of interface
	//for now, just create http default api
	if err := http.ListenAndServe("localhost:3000", s); err != nil {
		return fmt.Errorf("HTTP server failed: %v", err)
	}
	return nil
}

func (s *service) MustRun() {
	if err := s.Run(); err != nil {
		panic(err)
	}
}

func (s *service) ServeHTTP(httpRes http.ResponseWriter, httpReq *http.Request) {
	res := Response{
		Header: ResponseHeader{Success: false, Error: fmt.Sprintf("undefined error")},
		Data:   nil,
	}
	respondWithHeader := true
	defer func() {
		if respondWithHeader {
			jsonRes, err := json.Marshal(res)
			if err != nil {
				res = Response{
					Header: ResponseHeader{
						Success: false,
						Error:   fmt.Sprintf("failed to encode JSON response: %v", err),
					},
					Data: nil,
				}
				jsonRes, _ = json.Marshal(res)
			}
			httpRes.Header().Set("Content-Type", "application/json")
			httpRes.Write(jsonRes)
		}
	}()

	log.Debugf("HTTP %s %s", httpReq.Method, httpReq.URL.Path)

	//routing...
	routeMux, data := s.mux.Route(strings.Split(path.Clean(httpReq.URL.Path), "/"))
	log.Debugf("  %s -> hdlr(%+v),data(%+v)", httpReq.URL.Path, routeMux, data)
	if routeMux == nil || routeMux.Value() == nil {
		res.Header.Error = fmt.Sprintf("unknown route %s", httpReq.URL.Path)
		return
	}

	//if route value is an http handler, then call it and it has full control over response
	if httpHandlerFunc, ok := routeMux.Value().(http.HandlerFunc); ok {
		//full http handler function for any method
		//but give handler only the remaining path to care about

		httpReq.URL.Path = path.Clean(httpReq.URL.Path)[len(routeMux.Path("/")):]
		respondWithHeader = false
		httpHandlerFunc(httpRes, httpReq)
		return
	}

	//if route value is func(ctx IContext, muxData map[string]interface{}) (res interface{}, err error)
	if muxHandlerFunc, ok := routeMux.Value().(func(ctx IContext, muxData map[string]interface{}) (res interface{}, err error)); ok {
		//respondWithHeader = false
		var err error
		ctx := NewContext()
		res.Data, err = muxHandlerFunc(ctx, data)
		if err != nil {
			res.Header.Success = false
			res.Header.Error = fmt.Sprintf("handler failed: %v", err)
			return
		}
		res.Header.Success = true
		res.Header.Error = ""
		return
	}

	//if route value is a handler, then we implement generic request parsing and response encoding
	//but if not, we do not know what to do here...
	handler, ok := routeMux.Value().(handler)
	if !ok {
		res.Header.Error = fmt.Sprintf("unknown route value type %T", routeMux.Value())
		return
	}

	//customer request->response handler
	//parse URL into new req struct
	reqPtrValue := reflect.New(handler.reqType)
	for i := 0; i < handler.reqType.NumField(); i++ {
		f := handler.reqType.Field(i)
		fn := StructFieldDbColumnName(f)
		v := httpReq.URL.Query().Get(fn)
		if v != "" {
			switch f.Type.Kind() {
			case reflect.String:
				reqPtrValue.Elem().FieldByName(f.Name).Set(reflect.ValueOf(v))
			case reflect.Int:
				intValue, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					res.Header.Error = fmt.Sprintf("%s=%s is not integer value", fn, v)
					return
				}
				reqPtrValue.Elem().FieldByName(f.Name).Set(reflect.ValueOf(int(intValue)))

			default:
				panic(fmt.Errorf("cannot set type for %s:%v from URL value=%v", fn, f.Type, v))
			}

		}
	}

	for n, v := range httpReq.URL.Query() {
		f, ok := handler.reqType.FieldByNameFunc(
			func(name string) bool {
				log.Debugf("  match(%s)", name)
				return false
			},
		)
		if !ok {
			log.Debugf("field(%s) not found", n)
			continue
		}
		reqPtrValue.Elem().FieldByName(f.Name).Set(reflect.ValueOf(v[0]))
	}

	if validator, ok := reqPtrValue.Interface().(IValidator); ok {
		if err := validator.Validate(); err != nil {
			res.Header.Error = fmt.Sprintf("invalid request: %v", err)
			return
		}
	}

	log.Debugf("Request: %T: %+v", reqPtrValue.Elem().Interface(), reqPtrValue.Elem().Interface())

	ctx := NewContext()
	args := []reflect.Value{reflect.ValueOf(ctx), reqPtrValue.Elem()}
	log.Debugf("args=%+v", args)
	results := handler.fncValue.Call(args)
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		res.Header.Error = fmt.Sprintf("handler failed: %v", err)
		return
	}

	res.Header.Success = true
	res.Header.Error = ""
	res.Data = results[0].Interface()
	return
}

type HandlerFunc func(IContext, req interface{}) (res interface{}, err error)

type IContext interface {
	Debugf(format string, args ...interface{})
}

func NewContext() IContext {
	return context{}
}

type context struct{}

func (ctx context) Debugf(format string, args ...interface{}) {}

type Response struct {
	Header ResponseHeader `json:"header"`
	Data   interface{}    `json:"data,omitempty"`
}

type ResponseHeader struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type IValidator interface {
	Validate() error
}

//get name by which field is identified using json tag if specified
//else lowercase of field name
func StructFieldDbColumnName(f reflect.StructField) string {
	p := strings.Split(f.Tag.Get("json"), ",")
	if len(p) == 0 || p[0] == "" {
		return strings.ToLower(f.Name)
	}
	return p[0]
}
