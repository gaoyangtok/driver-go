package tmq

import (
	"fmt"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/taosdata/driver-go/v3/common/tmq"
	"github.com/taosdata/driver-go/v3/errors"
	"github.com/taosdata/driver-go/v3/wrapper"
)

func TestTmq(t *testing.T) {
	conn, err := wrapper.TaosConnect("", "root", "taosdata", "", 0)
	if err != nil {
		t.Error(err)
		return
	}
	sqls := []string{
		"drop topic if exists test_tmq_common",
		"drop database if exists af_test_tmq",
		"create database if not exists af_test_tmq vgroups 2  WAL_RETENTION_PERIOD 86400",
		"use af_test_tmq",
		"create stable if not exists all_type (ts timestamp," +
			"c1 bool," +
			"c2 tinyint," +
			"c3 smallint," +
			"c4 int," +
			"c5 bigint," +
			"c6 tinyint unsigned," +
			"c7 smallint unsigned," +
			"c8 int unsigned," +
			"c9 bigint unsigned," +
			"c10 float," +
			"c11 double," +
			"c12 binary(20)," +
			"c13 nchar(20)" +
			") tags(t1 int)",
		"create table if not exists ct0 using all_type tags(1000)",
		"create table if not exists ct1 using all_type tags(2000)",
		"create table if not exists ct2 using all_type tags(3000)",
		"create topic if not exists test_tmq_common as select ts,c1,c2,c3,c4,c5,c6,c7,c8,c9,c10,c11,c12,c13 from all_type",
	}

	defer func() {
		execWithoutResult(conn, "drop database if exists af_test_tmq")
	}()
	for _, sql := range sqls {
		err = execWithoutResult(conn, sql)
		assert.NoError(t, err)
	}
	defer func() {
		err = execWithoutResult(conn, "drop topic if exists test_tmq_common")
		assert.NoError(t, err)
	}()
	now := time.Now()
	err = execWithoutResult(conn, fmt.Sprintf("insert into ct0 values('%s',true,2,3,4,5,6,7,8,9,10,11,'1','2')", now.Format(time.RFC3339Nano)))
	assert.NoError(t, err)
	err = execWithoutResult(conn, fmt.Sprintf("insert into ct1 values('%s',true,2,3,4,5,6,7,8,9,10,11,'1','2')", now.Format(time.RFC3339Nano)))
	assert.NoError(t, err)
	err = execWithoutResult(conn, fmt.Sprintf("insert into ct2 values('%s',true,2,3,4,5,6,7,8,9,10,11,'1','2')", now.Format(time.RFC3339Nano)))
	assert.NoError(t, err)

	consumer, err := NewConsumer(&tmq.ConfigMap{
		"group.id":           "test",
		"auto.offset.reset":  "earliest",
		"td.connect.ip":      "127.0.0.1",
		"td.connect.user":    "root",
		"td.connect.pass":    "taosdata",
		"td.connect.port":    "6030",
		"client.id":          "test_tmq_c",
		"enable.auto.commit": "false",
		//"experimental.snapshot.enable": "true",
		"msg.with.table.name": "true",
	})
	if err != nil {
		t.Error(err)
		return
	}
	err = consumer.Subscribe("test_tmq_common", nil)
	if err != nil {
		t.Error(err)
		return
	}
	ass, err := consumer.Assignment()
	t.Log(ass)
	position, _ := consumer.Position(ass)
	t.Log(position)
	haveMessage := false
	for i := 0; i < 5; i++ {
		ev := consumer.Poll(500)
		if ev == nil {
			continue
		}
		haveMessage = true
		switch e := ev.(type) {
		case *tmq.DataMessage:
			row1 := e.Value().([]*tmq.Data)[0].Data[0]
			assert.Equal(t, "af_test_tmq", e.DBName())
			assert.Equal(t, now.UnixNano()/1e6, row1[0].(time.Time).UnixNano()/1e6)
			assert.Equal(t, true, row1[1].(bool))
			assert.Equal(t, int8(2), row1[2].(int8))
			assert.Equal(t, int16(3), row1[3].(int16))
			assert.Equal(t, int32(4), row1[4].(int32))
			assert.Equal(t, int64(5), row1[5].(int64))
			assert.Equal(t, uint8(6), row1[6].(uint8))
			assert.Equal(t, uint16(7), row1[7].(uint16))
			assert.Equal(t, uint32(8), row1[8].(uint32))
			assert.Equal(t, uint64(9), row1[9].(uint64))
			assert.Equal(t, float32(10), row1[10].(float32))
			assert.Equal(t, float64(11), row1[11].(float64))
			assert.Equal(t, "1", row1[12].(string))
			assert.Equal(t, "2", row1[13].(string))
			t.Log(e.Offset())
			ass, err := consumer.Assignment()
			t.Log(ass)
			committed, err := consumer.Committed(ass, 0)
			t.Log(committed)
			position, _ := consumer.Position(ass)
			t.Log(position)
			offsets, err := consumer.Position([]tmq.TopicPartition{e.TopicPartition})
			assert.NoError(t, err)
			consumer.CommitOffsets(offsets)
			assert.NoError(t, err)
			ass, err = consumer.Assignment()
			t.Log(ass)
			committed, err = consumer.Committed(ass, 0)
			t.Log(committed)
			position, _ = consumer.Position(ass)
			t.Log(position)
			err = consumer.Unsubscribe()
			assert.NoError(t, err)
			err = consumer.Close()
			assert.NoError(t, err)
			return
		case tmq.Error:
			t.Error(e)
			return
		default:
			t.Error("unexpected", e)
			return
		}
	}
	assert.True(t, haveMessage)
}

func TestSeek(t *testing.T) {
	conn, err := wrapper.TaosConnect("", "root", "taosdata", "", 0)
	if err != nil {
		t.Error(err)
		return
	}
	db := "af_test_tmq_seek"
	topic := "af_test_tmq_seek_topic"
	vgroups := 2
	record := 3
	sqls := []string{
		"drop topic if exists " + topic,
		"drop database if exists " + db,
		"create database if not exists " + db + " vgroups " + strconv.Itoa(vgroups) + "  WAL_RETENTION_PERIOD 86400",
		"use " + db,
		"create table stb(ts timestamp,v int) tags (n binary(10))",
		"create table ct0 using stb tags ('ct0')",
		"create table ct1 using stb tags ('ct1')",
		"create table ct3 using stb tags ('ct3')",
		"insert into ct0 values (now,0)",
		"insert into ct1 values (now,1)",
		"insert into ct3 values (now,2)",
		//"create topic " + topic + " as database " + db,
		//"create topic " + topic + " as select * from ct0 ",
		"create topic " + topic + " as select * from stb ",
	}

	defer func() {
		//execWithoutResult(conn, "drop database if exists "+db)
	}()
	for _, sql := range sqls {
		err = execWithoutResult(conn, sql)
		assert.NoError(t, err, sql)
	}
	defer func() {
		err = execWithoutResult(conn, "drop topic if exists "+topic)
		assert.NoError(t, err)
	}()
	consumer, err := NewConsumer(&tmq.ConfigMap{
		"group.id":            "test",
		"td.connect.ip":       "127.0.0.1",
		"td.connect.user":     "root",
		"td.connect.pass":     "taosdata",
		"td.connect.port":     "6030",
		"client.id":           "test_tmq_seek",
		"enable.auto.commit":  "false",
		"msg.with.table.name": "true",
	})
	if err != nil {
		t.Error(err)
		return
	}
	err = consumer.Subscribe(topic, nil)
	if err != nil {
		t.Error(err)
		return
	}
	assignment, err := consumer.Assignment()
	assert.NoError(t, err)
	assert.Equal(t, vgroups, len(assignment))
	for i := 0; i < len(assignment); i++ {
		assert.Equal(t, tmq.Offset(0), assignment[i].Offset)
		assert.Equal(t, topic, *assignment[i].Topic)
	}

	// poll
	dataCount := 0
	for i := 0; i < 20; i++ {
		if dataCount >= record {
			break
		}
		event := consumer.Poll(500)
		if event != nil {
			t.Log(event)
			data := event.(*tmq.DataMessage).Value().([]*tmq.Data)
			for _, datum := range data {
				dataCount += len(datum.Data)
			}
			_, err = consumer.Commit()
			assert.NoError(t, err)
		}
	}
	assert.Equal(t, record, dataCount)

	//assignment after poll
	assignment, err = consumer.Assignment()
	t.Log(assignment)
	assert.NoError(t, err)
	assert.Equal(t, vgroups, len(assignment))
	for i := 0; i < len(assignment); i++ {
		assert.Greater(t, assignment[i].Offset, tmq.Offset(0))
		assert.Equal(t, topic, *assignment[i].Topic)
	}

	// seek
	for i := 0; i < len(assignment); i++ {
		err = consumer.Seek(tmq.TopicPartition{
			Topic:     &topic,
			Partition: assignment[i].Partition,
			Offset:    0,
		}, 0)
		assert.NoError(t, err)
	}

	//assignment after seek
	assignment, err = consumer.Assignment()
	t.Log(assignment)
	assert.NoError(t, err)
	assert.Equal(t, vgroups, len(assignment))
	for i := 0; i < len(assignment); i++ {
		assert.Equal(t, tmq.Offset(0), assignment[i].Offset)
		assert.Equal(t, topic, *assignment[i].Topic)
	}

	//poll after seek
	dataCount = 0
	for i := 0; i < 20; i++ {
		if dataCount >= record {
			break
		}
		event := consumer.Poll(500)
		if event != nil {
			t.Log(event)
			data := event.(*tmq.DataMessage).Value().([]*tmq.Data)
			for _, datum := range data {
				dataCount += len(datum.Data)
			}
		}
		_, err = consumer.Commit()
		assert.NoError(t, err)
	}
	assert.Equal(t, record, dataCount)

	//assignment after poll
	assignment, err = consumer.Assignment()
	t.Log(assignment)
	assert.NoError(t, err)
	assert.Equal(t, vgroups, len(assignment))
	for i := 0; i < len(assignment); i++ {
		assert.Greater(t, assignment[i].Offset, tmq.Offset(0))
		assert.Equal(t, topic, *assignment[i].Topic)
	}
	consumer.Close()
}

func execWithoutResult(conn unsafe.Pointer, sql string) error {
	result := wrapper.TaosQuery(conn, sql)
	defer wrapper.TaosFreeResult(result)
	code := wrapper.TaosError(result)
	if code != 0 {
		errStr := wrapper.TaosErrorStr(result)
		wrapper.TaosFreeResult(result)
		return &errors.TaosError{Code: int32(code), ErrStr: errStr}
	}
	return nil
}
