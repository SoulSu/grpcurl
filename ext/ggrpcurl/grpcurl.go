// Command grpcurl makes gRPC requests (a la cURL, but HTTP/2). It can use a supplied descriptor
// file, protobuf sources, or service reflection to translate JSON or text request data into the
// appropriate protobuf messages and vice versa for presenting the response contents.
package ggrpcurl

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SoulSu/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	// Register gzip compressor so compressed responses will work
	_ "google.golang.org/grpc/encoding/gzip"
	// Register xds so xds and xds-experimental resolver schemes work
	_ "google.golang.org/grpc/xds"
)

// To avoid confusion between program error codes and the gRPC resonse
// status codes 'Cancelled' and 'Unknown', 1 and 2 respectively,
// the response status codes emitted use an offest of 64
const statusCodeOffset = 64

const no_version = "dev build <no version set>"

var version = "grpcurl"

//
//var (
//	exit = os.Exit
//
//	isUnixSocket func() bool // nil when run on non-unix platform
//
//	flags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
//
//	help = flags.Bool("help", false, prettify(`
//		Print usage instructions and exit.`))
//	printVersion = flags.Bool("version", false, prettify(`
//		Print version.`))
//	plaintext = flags.Bool("plaintext", false, prettify(`
//		Use plain-text HTTP/2 when connecting to server (no TLS).`))
//	insecure = flags.Bool("insecure", false, prettify(`
//		Skip server certificate and domain verification. (NOT SECURE!) Not
//		valid with -plaintext option.`))
//	cacert = flags.String("cacert", "", prettify(`
//		File containing trusted root certificates for verifying the server.
//		Ignored if -insecure is specified.`))
//	cert = flags.String("cert", "", prettify(`
//		File containing client certificate (public key), to present to the
//		server. Not valid with -plaintext option. Must also provide -key option.`))
//	key = flags.String("key", "", prettify(`
//		File containing client private key, to present to the server. Not valid
//		with -plaintext option. Must also provide -cert option.`))
//	protoset      multiString
//	protoFiles    multiString
//	importPaths   multiString
//	addlHeaders   multiString
//	rpcHeaders    multiString
//	reflHeaders   multiString
//	expandHeaders = flags.Bool("expand-headers", false, prettify(`
//		If set, headers may use '${NAME}' syntax to reference environment
//		variables. These will be expanded to the actual environment variable
//		value before sending to the server. For example, if there is an
//		environment variable defined like FOO=bar, then a header of
//		'key: ${FOO}' would expand to 'key: bar'. This applies to -H,
//		-rpc-header, and -reflect-header options. No other expansion/escaping is
//		performed. This can be used to supply credentials/secrets without having
//		to put them in command-line arguments.`))
//	authority = flags.String("authority", "", prettify(`
//		The authoritative name of the remote server. This value is passed as the
//		value of the ":authority" pseudo-header in the HTTP/2 protocol. When TLS
//		is used, this will also be used as the server name when verifying the
//		server's certificate. It defaults to the address that is provided in the
//		positional arguments.`))
//	userAgent = flags.String("user-agent", "", prettify(`
//		If set, the specified value will be added to the User-Agent header set
//		by the grpc-go library.
//		`))
//	data = flags.String("d", "", prettify(`
//		Data for request contents. If the value is '@' then the request contents
//		are read from stdin. For calls that accept a stream of requests, the
//		contents should include all such request messages concatenated together
//		(possibly delimited; see -format).`))
//	format = flags.String("format", "json", prettify(`
//		The format of request data. The allowed values are 'json' or 'text'. For
//		'json', the input data must be in JSON format. Multiple request values
//		may be concatenated (messages with a JSON representation other than
//		object must be separated by whitespace, such as a newline). For 'text',
//		the input data must be in the protobuf text format, in which case
//		multiple request values must be separated by the "record separator"
//		ASCII character: 0x1E. The stream should not end in a record separator.
//		If it does, it will be interpreted as a final, blank message after the
//		separator.`))
//	allowUnknownFields = flags.Bool("allow-unknown-fields", false, prettify(`
//		When true, the request contents, if 'json' format is used, allows
//		unkown fields to be present. They will be ignored when parsing
//		the request.`))
//	connectTimeout = flags.Float64("connect-timeout", 0, prettify(`
//		The maximum time, in seconds, to wait for connection to be established.
//		Defaults to 10 seconds.`))
//	formatError = flags.Bool("format-error", false, prettify(`
//		When a non-zero status is returned, format the response using the
//		value set by the -format flag .`))
//	keepaliveTime = flags.Float64("keepalive-time", 0, prettify(`
//		If present, the maximum idle time in seconds, after which a keepalive
//		probe is sent. If the connection remains idle and no keepalive response
//		is received for this same period then the connection is closed and the
//		operation fails.`))
//	maxTime = flags.Float64("max-time", 0, prettify(`
//		The maximum total time the operation can take, in seconds. This is
//		useful for preventing batch jobs that use grpcurl from hanging due to
//		slow or bad network links or due to incorrect stream method usage.`))
//	maxMsgSz = flags.Int("max-msg-sz", 0, prettify(`
//		The maximum encoded size of a response message, in bytes, that grpcurl
//		will accept. If not specified, defaults to 4,194,304 (4 megabytes).`))
//	emitDefaults = flags.Bool("emit-defaults", false, prettify(`
//		Emit default values for JSON-encoded responses.`))
//	protosetOut = flags.String("protoset-out", "", prettify(`
//		The name of a file to be written that will contain a FileDescriptorSet
//		proto. With the list and describe verbs, the listed or described
//		elements and their transitive dependencies will be written to the named
//		file if this option is given. When invoking an RPC and this option is
//		given, the method being invoked and its transitive dependencies will be
//		included in the output file.`))
//	msgTemplate = flags.Bool("msg-template", false, prettify(`
//		When describing messages, show a template of input data.`))
//	verbose = flags.Bool("v", false, prettify(`
//		Enable verbose output.`))
//	veryVerbose = flags.Bool("vv", false, prettify(`
//		Enable very verbose output.`))
//	serverName = flags.String("servername", "", prettify(`
//		Override server name when validating TLS certificate. This flag is
//		ignored if -plaintext or -insecure is used.
//		NOTE: Prefer -authority. This flag may be removed in the future. It is
//		an error to use both -authority and -servername (though this will be
//		permitted if they are both set to the same value, to increase backwards
//		compatibility with earlier releases that allowed both to be set).`))
//	reflection = optionalBoolFlag{val: true}
//)
//
//func init() {
//	flags.Var(&addlHeaders, "H", prettify(`
//		Additional headers in 'name: value' format. May specify more than one
//		via multiple flags. These headers will also be included in reflection
//		requests requests to a server.`))
//	flags.Var(&rpcHeaders, "rpc-header", prettify(`
//		Additional RPC headers in 'name: value' format. May specify more than
//		one via multiple flags. These headers will *only* be used when invoking
//		the requested RPC method. They are excluded from reflection requests.`))
//	flags.Var(&reflHeaders, "reflect-header", prettify(`
//		Additional reflection headers in 'name: value' format. May specify more
//		than one via multiple flags. These headers will *only* be used during
//		reflection requests and will be excluded when invoking the requested RPC
//		method.`))
//	flags.Var(&protoset, "protoset", prettify(`
//		The name of a file containing an encoded FileDescriptorSet. This file's
//		contents will be used to determine the RPC schema instead of querying
//		for it from the remote server via the gRPC reflection API. When set: the
//		'list' action lists the services found in the given descriptors (vs.
//		those exposed by the remote server), and the 'describe' action describes
//		symbols found in the given descriptors. May specify more than one via
//		multiple -protoset flags. It is an error to use both -protoset and
//		-proto flags.`))
//	flags.Var(&protoFiles, "proto", prettify(`
//		The name of a proto source file. Source files given will be used to
//		determine the RPC schema instead of querying for it from the remote
//		server via the gRPC reflection API. When set: the 'list' action lists
//		the services found in the given files and their imports (vs. those
//		exposed by the remote server), and the 'describe' action describes
//		symbols found in the given files. May specify more than one via multiple
//		-proto flags. Imports will be resolved using the given -import-path
//		flags. Multiple proto files can be specified by specifying multiple
//		-proto flags. It is an error to use both -protoset and -proto flags.`))
//	flags.Var(&importPaths, "import-path", prettify(`
//		The path to a directory from which proto sources can be imported, for
//		use with -proto flags. Multiple import paths can be configured by
//		specifying multiple -import-path flags. Paths will be searched in the
//		order given. If no import paths are given, all files (including all
//		imports) must be provided as -proto flags, and grpcurl will attempt to
//		resolve all import statements from the set of file names given.`))
//	flags.Var(&reflection, "use-reflection", prettify(`
//		When true, server reflection will be used to determine the RPC schema.
//		Defaults to true unless a -proto or -protoset option is provided. If
//		-use-reflection is used in combination with a -proto or -protoset flag,
//		the provided descriptor sources will be used in addition to server
//		reflection to resolve messages and extensions.`))
//}

type multiString []string

func (s *multiString) String() string {
	return strings.Join(*s, ",")
}

func (s *multiString) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Uses a file source as a fallback for resolving symbols and extensions, but
// only uses the reflection source for listing services
type compositeSource struct {
	reflection grpcurl.DescriptorSource
	file       grpcurl.DescriptorSource
}

func (cs compositeSource) ListServices() ([]string, error) {
	return cs.reflection.ListServices()
}

func (cs compositeSource) FindSymbol(fullyQualifiedName string) (desc.Descriptor, error) {
	d, err := cs.reflection.FindSymbol(fullyQualifiedName)
	if err == nil {
		return d, nil
	}
	return cs.file.FindSymbol(fullyQualifiedName)
}

func (cs compositeSource) AllExtensionsForType(typeName string) ([]*desc.FieldDescriptor, error) {
	exts, err := cs.reflection.AllExtensionsForType(typeName)
	if err != nil {
		// On error fall back to file source
		return cs.file.AllExtensionsForType(typeName)
	}
	// Track the tag numbers from the reflection source
	tags := make(map[int32]bool)
	for _, ext := range exts {
		tags[ext.GetNumber()] = true
	}
	fileExts, err := cs.file.AllExtensionsForType(typeName)
	if err != nil {
		return exts, nil
	}
	for _, ext := range fileExts {
		// Prioritize extensions found via reflection
		if !tags[ext.GetNumber()] {
			exts = append(exts, ext)
		}
	}
	return exts, nil
}

// GGrpCurlDTO 运行GRPCURL DTO
type GGrpCurlDTO struct {
	Plaintext     bool
	FormatError   bool
	EmitDefaults  bool
	AddHeaders    []string
	ImportPaths   []string
	ProtoFiles    []string
	Data          string // 请求数据
	ServiceAddr   string
	ServiceMethod string
	Trace         bool
}

type InvokeGRpc struct {
	connectTimeout     float64
	keepaliveTime      float64
	maxMsgSz           int
	plaintext          bool
	insecure           bool
	cacert             string
	cert               string
	key                string
	serverName         string
	authority          string
	userAgent          string
	data               string
	emitDefaults       bool
	allowUnknownFields bool
	format             string
	addlHeaders        []string
	rpcHeaders         []string
	formatError        bool
	maxTime            float64
	importPaths        []string
	protoFiles         []string

	serviceAddr   string
	serviceMethod string

	trace bool
}

var defaultInvokeGRpc = InvokeGRpc{
	connectTimeout:     0,
	keepaliveTime:      0,
	maxMsgSz:           0,
	plaintext:          false,
	insecure:           false,
	cacert:             "",
	cert:               "",
	key:                "",
	serverName:         "",
	authority:          "",
	userAgent:          "",
	data:               "",
	emitDefaults:       false,
	allowUnknownFields: false,
	format:             "json",
	addlHeaders:        nil,
	rpcHeaders:         nil,
	formatError:        false,
	maxTime:            0,
}

func NewInvokeGRpc(dto *GGrpCurlDTO) *InvokeGRpc {
	clone := defaultInvokeGRpc
	clone.plaintext = dto.Plaintext
	clone.formatError = dto.FormatError
	clone.emitDefaults = dto.EmitDefaults
	clone.addlHeaders = dto.AddHeaders
	clone.importPaths = dto.ImportPaths
	clone.protoFiles = dto.ProtoFiles
	clone.data = dto.Data
	clone.serviceAddr = dto.ServiceAddr
	clone.serviceMethod = dto.ServiceMethod
	clone.trace = dto.Trace
	return &clone
}

// Invoke 运行GGrpCurl 方法 直接发送请求
func (i *InvokeGRpc) Invoke() (metadata.MD, string, error) {

	target := i.serviceAddr
	symbol := i.serviceMethod
	verbosityLevel := 0
	if i.trace {
		verbosityLevel = 2
	}

	ctx := context.Background()
	if i.maxTime > 0 {
		timeout := time.Duration(i.maxTime * float64(time.Second))
		ctx, _ = context.WithTimeout(ctx, timeout)
	}

	dial := i.dial(ctx, target)
	printFormattedStatus := func(w io.Writer, stat *status.Status, formatter grpcurl.Formatter) {
		formattedStatus, err := formatter(stat.Proto())
		if err != nil {
			fmt.Fprintf(w, "ERROR: %v", err.Error())
		}
		fmt.Fprint(w, formattedStatus)
	}

	var cc *grpc.ClientConn
	var refClient *grpcreflect.Client
	var fileSource grpcurl.DescriptorSource

	var err error
	fileSource, err = grpcurl.DescriptorSourceFromProtoFiles(i.importPaths, i.protoFiles...)
	if err != nil {
		return nil, "", fail(err, "Failed to process proto source files.")
	}

	// arrange for the RPCs to be cleanly shutdown
	reset := func() {
		if refClient != nil {
			refClient.Reset()
			refClient = nil
		}
		if cc != nil {
			cc.Close()
			cc = nil
		}
	}
	defer reset()

	return i.invoke(cc, dial, verbosityLevel, fileSource, ctx, symbol, printFormattedStatus)
}

func (i *InvokeGRpc) dial(ctx context.Context, target string) func() *grpc.ClientConn {
	dial := func() *grpc.ClientConn {
		dialTime := 10 * time.Second
		if i.connectTimeout > 0 {
			dialTime = time.Duration(i.connectTimeout * float64(time.Second))
		}
		ctx, cancel := context.WithTimeout(ctx, dialTime)
		defer cancel()
		var opts []grpc.DialOption
		if i.keepaliveTime > 0 {
			timeout := time.Duration(i.keepaliveTime * float64(time.Second))
			opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    timeout,
				Timeout: timeout,
			}))
		}
		if i.maxMsgSz > 0 {
			opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(i.maxMsgSz)))
		}
		var creds credentials.TransportCredentials
		if !i.plaintext {
			var err error
			creds, err = grpcurl.ClientTransportCredentials(i.insecure, i.cacert, i.cert, i.key)
			if err != nil {
				fail(err, "Failed to configure transport credentials")
			}

			// can use either -servername or -authority; but not both
			if i.serverName != "" && i.authority != "" {
				if i.serverName == i.authority {
					warn("Both -servername and -authority are present; prefer only -authority.")
				} else {
					fail(nil, "Cannot specify different values for -servername and -authority.")
				}
			}
			overrideName := i.serverName
			if overrideName == "" {
				overrideName = i.authority
			}

			if overrideName != "" {
				if err := creds.OverrideServerName(overrideName); err != nil {
					fail(err, "Failed to override server name as %q", overrideName)
				}
			}
		} else if i.authority != "" {
			opts = append(opts, grpc.WithAuthority(i.authority))
		}

		grpcurlUA := "grpcurl/" + version
		if version == no_version {
			grpcurlUA = "grpcurl/dev-build (no version set)"
		}
		if i.userAgent != "" {
			grpcurlUA = i.userAgent + " " + grpcurlUA
		}
		opts = append(opts, grpc.WithUserAgent(grpcurlUA))

		network := "tcp"
		cc, err := grpcurl.BlockingDial(ctx, network, target, creds, opts...)
		if err != nil {
			fail(err, "Failed to dial target host %q", target)
		}
		return cc
	}
	return dial
}

func (i *InvokeGRpc) invoke(cc *grpc.ClientConn,
	dial func() *grpc.ClientConn,
	verbosityLevel int,
	descSource grpcurl.DescriptorSource,
	ctx context.Context,
	symbol string,
	printFormattedStatus func(w io.Writer, stat *status.Status, formatter grpcurl.Formatter),
) (metadata.MD, string, error) {
	// Invoke an RPC
	if cc == nil {
		cc = dial()
	}
	var in = strings.NewReader(i.data)
	// if not verbose output, then also include record delimiters
	// between each message, so output could potentially be piped
	// to another grpcurl process
	includeSeparators := verbosityLevel == 0
	options := grpcurl.FormatOptions{
		EmitJSONDefaultFields: i.emitDefaults,
		IncludeTextSeparator:  includeSeparators,
		AllowUnknownFields:    i.allowUnknownFields,
	}
	rf, formatter, err := grpcurl.RequestParserAndFormatter(grpcurl.Format(i.format), descSource, in, options)
	if err != nil {
		return nil, "", fail(err, "Failed to construct request parser and formatter for %q", i.format)
	}

	var outWrite bytes.Buffer

	h := &CustomEventHandler{
		DefaultEventHandler: &grpcurl.DefaultEventHandler{
			Out:            &outWrite,
			Debug:          os.Stdout,
			Formatter:      formatter,
			VerbosityLevel: verbosityLevel,
		},
	}
	err = grpcurl.InvokeRPC(ctx, descSource, cc, symbol, append(i.addlHeaders, i.rpcHeaders...), h, rf.Next)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && i.formatError {
			h.Status = errStatus
		} else {
			return nil, "", fail(err, "Error invoking method %q", symbol)
		}
	}
	reqSuffix := ""
	respSuffix := ""
	reqCount := rf.NumRequests()
	if reqCount != 1 {
		reqSuffix = "s"
	}
	if h.NumResponses != 1 {
		respSuffix = "s"
	}
	if verbosityLevel > 0 {
		fmt.Printf("Sent %d request%s and received %d response%s\n", reqCount, reqSuffix, h.NumResponses, respSuffix)
	}
	if h.Status.Code() != codes.OK {
		var errWrite bytes.Buffer
		printFormattedStatus(&errWrite, h.Status, formatter)
		return h.ResponseMd, errWrite.String(), nil
	}
	return h.ResponseMd, outWrite.String(), nil
}

func prettify(docString string) string {
	parts := strings.Split(docString, "\n")

	// cull empty lines and also remove trailing and leading spaces
	// from each line in the doc string
	j := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts[j] = part
		j++
	}

	return strings.Join(parts[:j], "\n"+indent())
}

func warn(msg string, args ...interface{}) {
	msg = fmt.Sprintf("Warning: %s\n", msg)
	fmt.Fprintf(os.Stderr, msg, args...)
}

func fail(err error, msg string, args ...interface{}) error {
	if err != nil {
		msg += ": %v"
		args = append(args, err)
	}
	return fmt.Errorf(msg, args...)
}
