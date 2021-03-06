package envoy

import (
	"bytes"
	"encoding/base64"
	"fmt"

	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyapi_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoyapi_route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoyapi_discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"

	xds_cache_types "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/ghodss/yaml"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	_ "github.com/cncf/udpa/go/udpa/annotations"
	_ "github.com/envoyproxy/go-control-plane/envoy/annotations"

	// the list of config imports have been generated executing the following command
	// in the go-control-plane repo:
	//
	// for pkg in $(dirname $(grep -R -l "package envoy_config") | egrep -v "v2alpha|v3|v1"); \
	//   do echo "_ \"github.com/envoyproxy/go-control-plane/${pkg}\""; done
	_ "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/cluster/redis"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/fault/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/buffer/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/compressor/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/cors/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/csrf/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/dynamo/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/ext_authz/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_http1_bridge/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/grpc_web/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/gzip/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/header_to_metadata/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/health_check/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/ip_tagging/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/lua/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/on_demand/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/router/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/squash/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/transcoder/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/http_inspector/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/original_dst/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/proxy_protocol/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/listener/tls_inspector/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/client_ssl_auth/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/direct_response/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/echo/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/ext_authz/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/mongo_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/redis_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/sni_cluster/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/health_checker/redis/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/listener/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/metrics/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/omit_canary_hosts/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/omit_host_metadata/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/previous_hosts/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/retry/previous_priorities"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/trace/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/transport_socket/raw_buffer/v2"
)

// Resources is a struct that holds the different envoy resources types
// so it can be deserialized directly from the yaml representation
type Resources struct {
	Clusters  []*envoyapi.Cluster  `protobuf:"bytes,2,rep,name=clusters,json=clusters" json:"clusters"`
	Listeners []*envoyapi.Listener `protobuf:"bytes,4,rep,name=listeners,json=listeners" json:"listeners"`
}

// Reset is noop function for resFromFile to implement protobuf interface
func (m *Resources) Reset() { *m = Resources{} }

// String is noop function for resFromFile to implement protobuf interface
func (m *Resources) String() string { return proto.CompactTextString(m) }

// ProtoMessage is noop function for resFromFile to implement protobuf interface
func (*Resources) ProtoMessage() {}

// YAMLtoResources -> DeserializeYAML([]byte(configMap.Data["config.yaml"]))
func YAMLtoResources(data []byte) (*Resources, error) {
	j, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("Error converting yaml to json: '%s'", err)
	}

	res := &Resources{}
	if err := jsonpb.Unmarshal(bytes.NewReader(j), res); err != nil {
		return nil, fmt.Errorf("Error deserializing resources: '%s'", err)
	}

	return res, nil
}

// ResourcesToJSON serializes a protobuf message into
// a json string
func ResourcesToJSON(pb proto.Message) ([]byte, error) {
	m := jsonpb.Marshaler{}

	json := bytes.NewBuffer([]byte{})
	err := m.Marshal(json, pb)
	if err != nil {
		return []byte{}, err
	}
	return json.Bytes(), nil
}

type ResourceMarshaller interface {
	Marshal(xds_cache_types.Resource) (string, error)
}

type ResourceUnmarshaller interface {
	Unmarshal(string, xds_cache_types.Resource) error
}

type JSON struct{}

func (s JSON) Marshal(res xds_cache_types.Resource) (string, error) {
	m := jsonpb.Marshaler{}

	json := bytes.NewBuffer([]byte{})
	err := m.Marshal(json, res)
	if err != nil {
		return "", err
	}
	return string(json.Bytes()), nil
}

func (s JSON) Unmarshal(str string, res xds_cache_types.Resource) error {

	var err error
	switch o := res.(type) {

	case *envoyapi.ClusterLoadAssignment:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	case *envoyapi.Cluster:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	case *envoyapi_route.Route:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	case *envoyapi.Listener:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	case *envoyapi_discovery.Runtime:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	case *envoyapi_auth.Secret:
		err = jsonpb.Unmarshal(bytes.NewReader([]byte(str)), o)

	default:
		err = fmt.Errorf("Unknown resource type")
	}

	if err != nil {
		return fmt.Errorf("Error deserializing resource: '%s'", err)
	}
	return nil
}

type B64JSON struct{}

func (s B64JSON) Unmarshal(str string, res xds_cache_types.Resource) error {
	b, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return fmt.Errorf("Error decoding base64 string: '%s'", err)
	}

	js := JSON{}
	err = js.Unmarshal(string(b), res)
	if err != nil {
		return err
	}

	return nil
}

type YAML struct{}

func (s YAML) Unmarshal(str string, res xds_cache_types.Resource) error {
	b, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return fmt.Errorf("Error converting yaml to json: '%s'", err)
	}

	js := JSON{}
	err = js.Unmarshal(string(b), res)
	if err != nil {
		return err
	}

	return nil
}
