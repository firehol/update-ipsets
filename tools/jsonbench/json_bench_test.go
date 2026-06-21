package jsonbench

import (
	"encoding/hex"
	stdjson "encoding/json"
	"fmt"
	"testing"

	sonic "github.com/bytedance/sonic"
	gojson "github.com/goccy/go-json"
	segmentjson "github.com/segmentio/encoding/json"
	veloxjson "github.com/velox-io/json"
)

type comparisonPairLedgerFile struct {
	Version          int                         `json:"version"`
	AlgorithmVersion int                         `json:"algorithm_version"`
	Entries          []comparisonPairLedgerEntry `json:"entries"`
}

type comparisonPairLedgerEntry struct {
	Left           string `json:"left"`
	Right          string `json:"right"`
	LeftHash       string `json:"left_hash,omitempty"`
	LeftHashValid  bool   `json:"left_hash_valid,omitempty"`
	RightHash      string `json:"right_hash,omitempty"`
	RightHashValid bool   `json:"right_hash_valid,omitempty"`
	Common         uint64 `json:"common"`
}

type jsonCodec struct {
	name      string
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

var comparisonPairLedgerJSONCodecs = []jsonCodec{
	{name: "encoding_json", marshal: stdjson.Marshal, unmarshal: stdjson.Unmarshal},
	{name: "sonic_config_default", marshal: sonic.ConfigDefault.Marshal, unmarshal: sonic.ConfigDefault.Unmarshal},
	{name: "sonic_config_std", marshal: sonic.ConfigStd.Marshal, unmarshal: sonic.ConfigStd.Unmarshal},
	{name: "goccy_go_json", marshal: gojson.Marshal, unmarshal: gojson.Unmarshal},
	{name: "segmentio_encoding_json", marshal: segmentjson.Marshal, unmarshal: segmentjson.Unmarshal},
	{
		name: "velox_io_json_default",
		marshal: func(v any) ([]byte, error) {
			return veloxjson.Marshal(v)
		},
		unmarshal: func(data []byte, v any) error {
			return veloxjson.Unmarshal(data, v)
		},
	},
	{
		name: "velox_io_json_safe_strings",
		marshal: func(v any) ([]byte, error) {
			return veloxjson.Marshal(v, veloxjson.WithStdCompat())
		},
		unmarshal: func(data []byte, v any) error {
			return veloxjson.Unmarshal(data, v, veloxjson.WithCopyString())
		},
	},
}

func BenchmarkComparisonPairLedgerJSONMarshal(b *testing.B) {
	payload := comparisonPairLedgerBenchmarkPayload(400)
	for _, codec := range comparisonPairLedgerJSONCodecs {
		b.Run(codec.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				data, err := codec.marshal(payload)
				if err != nil {
					b.Fatal(err)
				}
				jsonBenchBytes = data
			}
		})
	}
}

func BenchmarkComparisonPairLedgerJSONUnmarshal(b *testing.B) {
	payload := comparisonPairLedgerBenchmarkPayload(400)
	data, err := stdjson.Marshal(payload)
	if err != nil {
		b.Fatal(err)
	}
	for _, codec := range comparisonPairLedgerJSONCodecs {
		b.Run(codec.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var got comparisonPairLedgerFile
				if err := codec.unmarshal(data, &got); err != nil {
					b.Fatal(err)
				}
				if len(got.Entries) != len(payload.Entries) {
					b.Fatalf("entry count = %d, want %d", len(got.Entries), len(payload.Entries))
				}
				jsonBenchPayload = got
			}
		})
	}
}

func comparisonPairLedgerBenchmarkPayload(feeds int) comparisonPairLedgerFile {
	entries := make([]comparisonPairLedgerEntry, 0, feeds*(feeds-1)/2)
	hashes := make([]string, feeds)
	for i := range hashes {
		var sum [32]byte
		sum[0] = byte(i >> 24)
		sum[1] = byte(i >> 16)
		sum[2] = byte(i >> 8)
		sum[3] = byte(i)
		hashes[i] = hex.EncodeToString(sum[:])
	}
	for i := 0; i < feeds; i++ {
		for j := i + 1; j < feeds; j++ {
			entries = append(entries, comparisonPairLedgerEntry{
				Left:           fmt.Sprintf("feed_%03d", i),
				Right:          fmt.Sprintf("feed_%03d", j),
				LeftHash:       hashes[i],
				LeftHashValid:  true,
				RightHash:      hashes[j],
				RightHashValid: true,
				Common:         1,
			})
		}
	}
	return comparisonPairLedgerFile{
		Version:          1,
		AlgorithmVersion: 1,
		Entries:          entries,
	}
}

var (
	jsonBenchBytes   []byte
	jsonBenchPayload comparisonPairLedgerFile
)
