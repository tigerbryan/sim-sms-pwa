package version

import "runtime"

var (
	Version   = "1.0.0"
	Commit    = "dev"
	BuildTime = "unknown"
)

const (
	QMIGoVersion    = "v0.6.4"
	SMSDecoderVersion = "v1.0.0"
)

type Info struct {
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	BuildTime       string `json:"build_time"`
	GoVersion       string `json:"go_version"`
	QMIGoVersion    string `json:"qmi_go_version"`
	SMSDecoderVersion string `json:"sms_decoder_version"`
	License         string `json:"license"`
	SMSOnly         bool   `json:"sms_only"`
}

func Current() Info {
	return Info{
		Version: Version, Commit: Commit, BuildTime: BuildTime,
		GoVersion: runtime.Version(), QMIGoVersion: QMIGoVersion,
		SMSDecoderVersion: SMSDecoderVersion, License: "MIT", SMSOnly: true,
	}
}
