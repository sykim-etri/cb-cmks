// Package config - Configuration for Cloud-Barista's GRPC and provides the required process
package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// ===== [ Constants and Variables ] =====

const (
	// ConfigVersion - 설정 구조에 대한 버전
	ConfigVersion = 1
)

// ===== [ Types ] =====

// GrpcConfig - CB-GRPC 서비스 설정 구조
type GrpcConfig struct {
	Version int             `mapstructure:"version"`
	GSL     GrpcServiceList `mapstructure:"grpc"`
}

// GrpcServiceList - CB-GRPC 서비스 목록
type GrpcServiceList struct {
	MCKSSrv *GrpcServerConfig `mapstructure:"mckssrv"`
	MCKSCli *GrpcClientConfig `mapstructure:"mckscli"`
}

// GrpcServerConfig - CB-GRPC 서버 설정 구조
type GrpcServerConfig struct {
	Addr         string              `mapstructure:"addr"`
	Reflection   string              `mapstructure:"reflection"`
	TLS          *TLSConfig          `mapstructure:"tls"`
	Interceptors *InterceptorsConfig `mapstructure:"interceptors"`
}

// GrpcClientConfig - CB-GRPC 클라이언트 설정 구조
type GrpcClientConfig struct {
	ServerAddr   string              `mapstructure:"server_addr"`
	Timeout      time.Duration       `mapstructure:"timeout"`
	TLS          *TLSConfig          `mapstructure:"tls"`
	Interceptors *InterceptorsConfig `mapstructure:"interceptors"`
}

// TLSConfig - TLS 설정 구조
type TLSConfig struct {
	TLSCert string `mapstructure:"tls_cert"`
	TLSKey  string `mapstructure:"tls_key"`
	TLSCA   string `mapstructure:"tls_ca"`
}

// InterceptorsConfig - GRPC 인터셉터 설정 구조
type InterceptorsConfig struct {
	AuthJWT           *AuthJWTConfig           `mapstructure:"auth_jwt"`
	PrometheusMetrics *PrometheusMetricsConfig `mapstructure:"prometheus_metrics"`
	Opentracing       *OpentracingConfig       `mapstructure:"opentracing"`
}

// AuthJWTConfig - AuthJWT 설정 구조
type AuthJWTConfig struct {
	JWTKey   string `mapstructure:"jwt_key"`
	JWTToken string `mapstructure:"jwt_token"`
}

// PrometheusMetricsConfig - Prometheus Metrics 설정 구조
type PrometheusMetricsConfig struct {
	ListenPort int `mapstructure:"listen_port"`
}

// OpentracingConfig - Opentracing 설정 구조
type OpentracingConfig struct {
	Jaeger *JaegerClientConfig `mapstructure:"jaeger"`
}

// JaegerClientConfig - Jaeger Client 설정 구조
type JaegerClientConfig struct {
	Endpoint    string  `mapstructure:"endpoint"`
	ServiceName string  `mapstructure:"service_name"`
	SampleRate  float64 `mapstructure:"sample_rate"`
}

// UnsupportedVersionError - 설정 초기화 과정에서 버전 검증을 통해 반환할 오류 구조
type UnsupportedVersionError struct {
	Have int
	Want int
}

// ===== [ Implementations ] =====

// Init - 설정에 대한 검사 및 초기화
func (gConf *GrpcConfig) Init() error {
	// 설정 파일 버전 검증
	if gConf.Version != ConfigVersion {
		return &UnsupportedVersionError{
			Have: gConf.Version,
			Want: ConfigVersion,
		}
	}
	// 전역변수 초기화
	gConf.initGlobalParams()

	return nil
}

// initGlobalParams - 전역 설정 초기화
func (gConf *GrpcConfig) initGlobalParams() {

	if gConf.GSL.MCKSSrv != nil {

		if gConf.GSL.MCKSSrv.TLS != nil {
			if gConf.GSL.MCKSSrv.TLS.TLSCert != "" {
				gConf.GSL.MCKSSrv.TLS.TLSCert = ReplaceEnvPath(gConf.GSL.MCKSSrv.TLS.TLSCert)
			}
			if gConf.GSL.MCKSSrv.TLS.TLSKey != "" {
				gConf.GSL.MCKSSrv.TLS.TLSKey = ReplaceEnvPath(gConf.GSL.MCKSSrv.TLS.TLSKey)
			}
		}

		if gConf.GSL.MCKSSrv.Interceptors != nil {
			if gConf.GSL.MCKSSrv.Interceptors.Opentracing != nil {
				if gConf.GSL.MCKSSrv.Interceptors.Opentracing.Jaeger != nil {

					if gConf.GSL.MCKSSrv.Interceptors.Opentracing.Jaeger.ServiceName == "" {
						gConf.GSL.MCKSSrv.Interceptors.Opentracing.Jaeger.ServiceName = "grpc mcks server"
					}

					if gConf.GSL.MCKSSrv.Interceptors.Opentracing.Jaeger.SampleRate == 0 {
						gConf.GSL.MCKSSrv.Interceptors.Opentracing.Jaeger.SampleRate = 1
					}

				}
			}
		}
	}

	if gConf.GSL.MCKSCli != nil {

		if gConf.GSL.MCKSCli.Timeout == 0 {
			gConf.GSL.MCKSCli.Timeout = 90 * time.Second
		}

		if gConf.GSL.MCKSCli.TLS != nil {
			if gConf.GSL.MCKSCli.TLS.TLSCA != "" {
				gConf.GSL.MCKSCli.TLS.TLSCA = ReplaceEnvPath(gConf.GSL.MCKSCli.TLS.TLSCA)
			}
		}

		if gConf.GSL.MCKSCli.Interceptors != nil {
			if gConf.GSL.MCKSCli.Interceptors.Opentracing != nil {
				if gConf.GSL.MCKSCli.Interceptors.Opentracing.Jaeger != nil {

					if gConf.GSL.MCKSCli.Interceptors.Opentracing.Jaeger.ServiceName == "" {
						gConf.GSL.MCKSCli.Interceptors.Opentracing.Jaeger.ServiceName = "grpc dragonfly client"
					}

					if gConf.GSL.MCKSCli.Interceptors.Opentracing.Jaeger.SampleRate == 0 {
						gConf.GSL.MCKSCli.Interceptors.Opentracing.Jaeger.SampleRate = 1
					}

				}
			}
		}
	}

}

// Error - 비 호환 버전에 대한 오류 문자열 반환
func (u *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("Unsupported version: %d (wanted: %d)", u.Have, u.Want)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// ReplaceEnvPath - $ABC/def ==> /abc/def
func ReplaceEnvPath(str string) string {
	if strings.Index(str, "$") == -1 {
		return str
	}

	// ex) input "$CBSTORE_ROOT/meta_db/dat"
	strList := strings.Split(str, "/")
	for n, one := range strList {
		if strings.Index(one, "$") != -1 {
			cbstoreRootPath := os.Getenv(strings.Trim(one, "$"))
			if cbstoreRootPath == "" {
				log.Fatal(one + " is not set!")
			}
			strList[n] = cbstoreRootPath
		}
	}

	var resultStr string
	for _, one := range strList {
		resultStr = resultStr + one + "/"
	}
	// ex) "/root/go/src/github.com/cloud-barista/cb-spider/meta_db/dat/"
	resultStr = strings.TrimRight(resultStr, "/")
	resultStr = strings.ReplaceAll(resultStr, "//", "/")
	return resultStr
}
