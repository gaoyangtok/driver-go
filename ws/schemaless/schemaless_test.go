package schemaless

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	taosErrors "github.com/taosdata/driver-go/v3/errors"
	"github.com/taosdata/driver-go/v3/ws/client"
)

// @author: xftan
// @date: 2023/10/13 11:35
// @description: test websocket schemaless insert
func TestSchemaless_Insert(t *testing.T) {
	cases := []struct {
		name      string
		protocol  int
		precision string
		data      string
		ttl       int
		code      int
	}{
		{
			name:      "influxdb",
			protocol:  InfluxDBLineProtocol,
			precision: "ms",
			data: "measurement,host=host1 field1=2i,field2=2.0 1577837300000\n" +
				"measurement,host=host1 field1=2i,field2=2.0 1577837400000\n" +
				"measurement,host=host1 field1=2i,field2=2.0 1577837500000\n" +
				"measurement,host=host1 field1=2i,field2=2.0 1577837600000",
			ttl: 1000,
		},
		{
			name:      "opentsdb_telnet",
			protocol:  OpenTSDBTelnetLineProtocol,
			precision: "ms",
			data: "meters.current 1648432611249 10.3 location=California.SanFrancisco group=2\n" +
				"meters.current 1648432611250 12.6 location=California.SanFrancisco group=2\n" +
				"meters.current 1648432611251 10.8 location=California.LosAngeles group=3\n" +
				"meters.current 1648432611252 11.3 location=California.LosAngeles group=3\n",
			ttl: 1000,
		},
		{
			name:      "opentsdb_json",
			protocol:  OpenTSDBJsonFormatProtocol,
			precision: "ms",
			data: "[{\"metric\": \"meters.voltage\", \"timestamp\": 1648432611249, \"value\": 219, \"tags\": " +
				"{\"location\": \"California.LosAngeles\", \"groupid\": 1 } }, {\"metric\": \"meters.voltage\", " +
				"\"timestamp\": 1648432611250, \"value\": 221, \"tags\": {\"location\": \"California.LosAngeles\", " +
				"\"groupid\": 1 } }]",
			ttl: 100,
		},
	}

	if err := before(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = after() }()

	s, err := NewSchemaless(NewConfig("ws://localhost:6041", 1,
		SetDb("test_schemaless_ws"),
		SetReadTimeout(10*time.Second),
		SetWriteTimeout(10*time.Second),
		SetUser("root"),
		SetPassword("taosdata"),
		SetEnableCompression(true),
		SetErrorHandler(func(err error) {
			t.Fatal(err)
		}),
	))
	if err != nil {
		t.Fatal(err)
	}
	//defer s.Close()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := s.Insert(c.data, c.protocol, c.precision, c.ttl, 0); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func doRequest(sql string) error {
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:6041/rest/sql", strings.NewReader(sql))
	req.Header.Set("Authorization", "Taosd /KfeAzX/f9na8qdtNZmtONryp201ma04bEl8LcvLUd7a8qdtNZmtONryp201ma04")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http code: %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	iter := client.JsonI.BorrowIterator(data)
	code := int32(0)
	desc := ""
	iter.ReadObjectCB(func(iter *jsoniter.Iterator, s string) bool {
		switch s {
		case "code":
			code = iter.ReadInt32()
		case "desc":
			desc = iter.ReadString()
		default:
			iter.Skip()
		}
		return iter.Error == nil
	})
	client.JsonI.ReturnIterator(iter)
	if code != 0 {
		return taosErrors.NewError(int(code), desc)
	}
	return nil
}

func before() error {
	if err := doRequest("drop database if exists test_schemaless_ws"); err != nil {
		return err
	}
	return doRequest("create database if not exists test_schemaless_ws")
}

func after() error {
	return doRequest("drop database  test_schemaless_ws")
}
