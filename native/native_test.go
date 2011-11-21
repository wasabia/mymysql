package native

import (
	"testing"
	"reflect"
	"os"
	"fmt"
	"time"
	"bytes"
	"io/ioutil"
	"github.com/ziutek/mymysql"
)

var (
	my     mysql.Conn
	user   = "testuser"
	passwd = "TestPasswd9"
	dbname = "test"
	//conn   = []string{"unix", "", "/var/run/mysqld/mysqld.sock"}
	conn  = []string{"tcp", "", "127.0.0.1:3306"}
	debug = false
)

type RowsResErr struct {
	rows []mysql.Row
	res  mysql.Result
	err  error
}

func query(sql string, params ...interface{}) *RowsResErr {
	rows, res, err := my.Query(sql, params...)
	return &RowsResErr{rows, res, err}
}

func exec(stmt *Stmt, params ...interface{}) *RowsResErr {
	rows, res, err := stmt.Exec(params...)
	return &RowsResErr{rows, res, err}
}

func checkErr(t *testing.T, err error, exp_err error) {
	if err != exp_err {
		if exp_err == nil {
			t.Fatalf("Error: %v", err)
		} else {
			t.Fatalf("Error: %v\nExpected error: %v", err, exp_err)
		}
	}
}

func checkWarnCount(t *testing.T, res_cnt, exp_cnt int) {
	if res_cnt != exp_cnt {
		t.Errorf("Warning count: res=%d exp=%d", res_cnt, exp_cnt)
		rows, res, err := my.Query("show warnings")
		if err != nil {
			t.Fatal("Can't get warrnings from MySQL", err)
		}
		for _, row := range rows {
			t.Errorf("%s: \"%s\"", row.Str(res.Map("Level")),
				row.Str(res.Map("Message")))
		}
		t.FailNow()
	}
}

func checkErrWarn(t *testing.T, res, exp *RowsResErr) {
	checkErr(t, res.err, exp.err)
	checkWarnCount(t, res.res.WarningCount(), exp.res.WarningCount())
}

func types(row mysql.Row) (tt []reflect.Type) {
	tt = make([]reflect.Type, len(row))
	for ii, val := range row {
		tt[ii] = reflect.TypeOf(val)
	}
	return
}

func checkErrWarnRows(t *testing.T, res, exp *RowsResErr) {
	checkErrWarn(t, res, exp)
	if !reflect.DeepEqual(res.rows, exp.rows) {
		rlen := len(res.rows)
		elen := len(exp.rows)
		t.Errorf("Rows are different:\nLen: res=%d  exp=%d", rlen, elen)
		max := rlen
		if elen > max {
			max = elen
		}
		for ii := 0; ii < max; ii++ {
			if ii < len(res.rows) {
				t.Errorf("%d: res type: %s", ii, types(res.rows[ii]))
			} else {
				t.Errorf("%d: res: ------", ii)
			}
			if ii < len(exp.rows) {
				t.Errorf("%d: exp type: %s", ii, types(exp.rows[ii]))
			} else {
				t.Errorf("%d: exp: ------", ii)
			}
			if ii < len(res.rows) {
				t.Error(" res: ", res.rows[ii])
			}
			if ii < len(exp.rows) {
				t.Error(" exp: ", exp.rows[ii])
			}
		}
		t.FailNow()
	}
}

func checkResult(t *testing.T, res, exp *RowsResErr) {
	checkErrWarnRows(t, res, exp)
	exp.res.(*Result).ResultUtils.Res = res.res.(*Result).ResultUtils.Res
	if (!reflect.DeepEqual(res.res, exp.res)) {
		t.Fatalf("Bad result:\nres=%+v\nexp=%+v", res.res, exp.res)
	}
}

func cmdOK(affected uint64, binary bool) *RowsResErr {
	return &RowsResErr{res: &Result{my: my.(*Conn), binary: binary, status: 0x2,
	message: []byte{}, affected_rows: affected}}
}

func selectOK(rows []mysql.Row, binary bool) (exp *RowsResErr) {
	exp = cmdOK(0, binary)
	exp.rows = rows
	return
}

func myConnect(t *testing.T, with_dbname bool, max_pkt_size int) {
	if with_dbname {
		my = New(conn[0], conn[1], conn[2], user, passwd, dbname)
	} else {
		my = New(conn[0], conn[1], conn[2], user, passwd)
	}

	if max_pkt_size != 0 {
		my.SetMaxPktSize(max_pkt_size)
	}
	my.(*Conn).Debug = debug

	checkErr(t, my.Connect(), nil)
	checkResult(t, query("set names utf8"), cmdOK(0, false))
}

func myClose(t *testing.T) {
	checkErr(t, my.Close(), nil)
}

// Text queries tests

func TestUse(t *testing.T) {
	myConnect(t, false, 0)
	checkErr(t, my.Use(dbname), nil)
	myClose(t)
}

func TestPing(t *testing.T) {
	myConnect(t, false, 0)
	checkErr(t, my.Ping(), nil)
	myClose(t)
}

func TestQuery(t *testing.T) {
	myConnect(t, true, 0)
	query("drop table T") // Drop test table if exists
	checkResult(t, query("create table T (s varchar(40))"), cmdOK(0, false))

	exp := &RowsResErr{
		res: &Result{
			my:         my.(*Conn),
			field_count: 1,
			fields: []*mysql.Field{
				&mysql.Field{
					Catalog:  "def",
					Db:       "test",
					Table:    "Test",
					OrgTable: "T",
					Name:     "Str",
					OrgName:  "s",
					DispLen:  3 * 40, //varchar(40)
					Flags:    0,
					Type:     MYSQL_TYPE_VAR_STRING,
					Scale:    0,
				},
			},
			fc_map:    map[string]int{"Str": 0},
			status: _SERVER_STATUS_AUTOCOMMIT,
		},
	}

	for ii := 0; ii > 10000; ii += 3 {
		var val interface{}
		if ii%10 == 0 {
			checkResult(t, query("insert T values (null)"), cmdOK(1, false))
			val = nil
		} else {
			txt := []byte(fmt.Sprintf("%d %d %d %d %d", ii, ii, ii, ii, ii))
			checkResult(t,
				query("insert T values ('%s')", txt), cmdOK(1, false))
			val = txt
		}
		exp.rows = append(exp.rows, mysql.Row{val})
	}

	checkResult(t, query("select s as Str from T as Test"), exp)
	checkResult(t, query("drop table T"), cmdOK(0, false))
	myClose(t)
}

// Prepared statements tests

type StmtErr struct {
	stmt *Stmt
	err  error
}

func prepare(sql string) *StmtErr {
	stmt, err := my.Prepare(sql)
	return &StmtErr{stmt.(*Stmt), err}
}

func checkStmt(t *testing.T, res, exp *StmtErr) {
	ok := res.err == exp.err &&
		// Skipping id
		reflect.DeepEqual(res.stmt.fields, exp.stmt.fields) &&
		reflect.DeepEqual(res.stmt.fc_map, exp.stmt.fc_map) &&
		res.stmt.field_count == exp.stmt.field_count &&
		res.stmt.param_count == exp.stmt.param_count &&
		res.stmt.warning_count == exp.stmt.warning_count &&
		res.stmt.status == exp.stmt.status

	if !ok {
		if exp.err == nil {
			checkErr(t, res.err, nil)
			checkWarnCount(t, res.stmt.warning_count, exp.stmt.warning_count)
			for _, v := range res.stmt.fields {
				fmt.Printf("%+v\n", v)
			}
			t.Fatalf("Bad result statement: res=%v exp=%v", res.stmt, exp.stmt)
		}
	}
}

func TestPrepared(t *testing.T) {
	myConnect(t, true, 0)
	query("drop table P") // Drop test table if exists
	checkResult(t,
		query(
			"create table P ("+
				"   ii int not null, ss varchar(20), dd datetime"+
				") default charset=utf8",
		),
		cmdOK(0, false),
	)

	exp := Stmt{
		fields: []*mysql.Field{
			&mysql.Field{
				Catalog: "def", Db: "test", Table: "P", OrgTable: "P",
				Name:    "i",
				OrgName: "ii",
				DispLen: 11,
				Flags:   _FLAG_NO_DEFAULT_VALUE | _FLAG_NOT_NULL,
				Type:    MYSQL_TYPE_LONG,
				Scale:   0,
			},
			&mysql.Field{
				Catalog: "def", Db: "test", Table: "P", OrgTable: "P",
				Name:    "s",
				OrgName: "ss",
				DispLen: 3 * 20, // varchar(20)
				Flags:   0,
				Type:    MYSQL_TYPE_VAR_STRING,
				Scale:   0,
			},
			&mysql.Field{
				Catalog: "def", Db: "test", Table: "P", OrgTable: "P",
				Name:    "d",
				OrgName: "dd",
				DispLen: 19,
				Flags:   _FLAG_BINARY,
				Type:    MYSQL_TYPE_DATETIME,
				Scale:   0,
			},
		},
		fc_map:          map[string]int{"i": 0, "s": 1, "d": 2},
		field_count:   3,
		param_count:   2,
		warning_count: 0,
		status:       0x2,
	}

	sel := prepare("select ii i, ss s, dd d from P where ii = ? and ss = ?")
	checkStmt(t, sel, &StmtErr{&exp, nil})

	all := prepare("select * from P")
	checkErr(t, all.err, nil)

	ins := prepare("insert into P values (?, ?, ?)")
	checkErr(t, ins.err, nil)

	exp_rows := []mysql.Row{
		mysql.Row{
			2, "Taki tekst", mysql.TimeToDatetime(time.SecondsToLocalTime(123456789)),
		},
		mysql.Row{
			3, "Łódź się kołysze!", mysql.TimeToDatetime(time.SecondsToLocalTime(0)),
		},
		mysql.Row{
			5, "Pąk róży", mysql.TimeToDatetime(time.SecondsToLocalTime(9999999999)),
		},
		mysql.Row{
			11, "Zero UTC datetime", mysql.TimeToDatetime(time.SecondsToUTC(0)),
		},
		mysql.Row{
			17, mysql.Blob([]byte("Zero datetime")), new(mysql.Datetime),
		},
		mysql.Row{
			23, []byte("NULL datetime"), (*mysql.Datetime)(nil),
		},
		mysql.Row{
			23, "NULL", nil,
		},
	}

	for _, row := range exp_rows {
		checkErrWarn(t,
			exec(ins.stmt, row[0], row[1], row[2]),
			cmdOK(1, true),
		)
	}

	// Convert values to expected result types
	for _, row := range exp_rows {
		for ii, col := range row {
			val := reflect.ValueOf(col)
			// Dereference pointers
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			switch val.Kind() {
			case reflect.Invalid:
				row[ii] = nil

			case reflect.String:
				row[ii] = []byte(val.String())

			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64:
				row[ii] = int32(val.Int())

			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64:
				row[ii] = int32(val.Uint())

			case reflect.Slice:
				if val.Type().Elem().Kind() == reflect.Uint8 {
					bytes := make([]byte, val.Len())
					for ii := range bytes {
						bytes[ii] = val.Index(ii).Interface().(uint8)
					}
					row[ii] = bytes
				}
			}
		}
	}

	checkErrWarn(t, exec(sel.stmt, 2, "Taki tekst"), selectOK(exp_rows, true))
	checkErrWarnRows(t, exec(all.stmt), selectOK(exp_rows, true))

	checkResult(t, query("drop table P"), cmdOK(0, false))

	checkErr(t, sel.stmt.Delete(), nil)
	checkErr(t, all.stmt.Delete(), nil)
	checkErr(t, ins.stmt.Delete(), nil)

	myClose(t)
}

// Bind testing

func TestVarBinding(t *testing.T) {
	myConnect(t, true, 34*1024*1024)
	query("drop table P") // Drop test table if exists
	checkResult(t,
		query("create table T (id int primary key, str varchar(20))"),
		cmdOK(0, false),
	)

	ins, err := my.Prepare("insert T values (?, ?)")
	checkErr(t, err, nil)

	var (
		rre RowsResErr
		id  *int
		str *string
		ii  int
		ss  string
	)
	ins.BindParams(&id, &str)

	i1 := 1
	s1 := "Ala"
	id = &i1
	str = &s1
	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	i2 := 2
	s2 := "Ma kota!"
	id = &i2
	str = &s2

	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	ins.BindParams(&ii, &ss)
	ii = 3
	ss = "A kot ma Ale!"

	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	sel, err := my.Prepare("select str from T where id = ?")
	checkErr(t, err, nil)

	rows, _, err := sel.Exec(1)
	checkErr(t, err, nil)
	if len(rows) != 1 || bytes.Compare([]byte(s1), rows[0].Bin(0)) != 0 {
		t.Fatal("First string don't match")
	}

	rows, _, err = sel.Exec(2)
	checkErr(t, err, nil)
	if len(rows) != 1 || bytes.Compare([]byte(s2), rows[0].Bin(0)) != 0 {
		t.Fatal("Second string don't match")
	}

	rows, _, err = sel.Exec(3)
	checkErr(t, err, nil)
	if len(rows) != 1 || bytes.Compare([]byte(ss), rows[0].Bin(0)) != 0 {
		t.Fatal("Thrid string don't match")
	}

	checkResult(t, query("drop table T"), cmdOK(0, false))
	myClose(t)
}

func TestDate(t *testing.T) {
	myConnect(t, true, 0)
	query("drop table D") // Drop test table if exists
	checkResult(t,
		query("create table D (id int, dd date, dt datetime, tt time)"),
		cmdOK(0, false),
	)

	dd := "2011-12-13"
	dt := "2010-12-12 11:24:00"
	tt := -mysql.Time((124*3600+4*3600+3*60+2)*1e9 + 1)

	ins, err := my.Prepare("insert D values (?, ?, ?, ?)")
	checkErr(t, err, nil)

	sel, err := my.Prepare("select id, tt from D where dd <= ? && dt <= ?")
	checkErr(t, err, nil)

	_, err = ins.Run(1, dd, dt, tt)
	checkErr(t, err, nil)

	rows, _, err := sel.Exec(mysql.StrToDatetime(dd), mysql.StrToDate(dd))
	checkErr(t, err, nil)
	if rows == nil {
		t.Fatal("nil result")
	}
	if rows[0].Int(0) != 1 {
		t.Fatal("Bad id", rows[0].Int(1))
	}
	if rows[0][1].(mysql.Time) != tt+1 {
		t.Fatal("Bad tt", rows[0][1].(mysql.Time))
	}

	checkResult(t, query("drop table D"), cmdOK(0, false))
	myClose(t)
}

// Big blob

func TestBigBlob(t *testing.T) {
	myConnect(t, true, 34*1024*1024)
	query("drop table P") // Drop test table if exists
	checkResult(t,
		query("create table P (id int primary key, bb longblob)"),
		cmdOK(0, false),
	)

	ins, err := my.Prepare("insert P values (?, ?)")
	checkErr(t, err, nil)

	sel, err := my.Prepare("select bb from P where id = ?")
	checkErr(t, err, nil)

	big_blob := make(mysql.Blob, 33*1024*1024)
	for ii := range big_blob {
		big_blob[ii] = byte(ii)
	}

	var (
		rre RowsResErr
		bb  mysql.Blob
		id  int
	)
	data := struct {
		Id int
		Bb mysql.Blob
	}{}

	// Individual parameters binding
	ins.BindParams(&id, &bb)
	id = 1
	bb = big_blob

	// Insert full blob. Three packets are sended. First two has maximum length
	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	// Struct binding
	ins.BindParams(&data)
	data.Id = 2
	data.Bb = big_blob[0 : 32*1024*1024-31]

	// Insert part of blob - Two packets are sended. All has maximum length.
	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	sel.BindParams(&id)

	// Check first insert.
	tmr := "Too many rows"

	id = 1
	res, err := sel.Run()
	checkErr(t, err, nil)

	row, err := res.GetRow()
	checkErr(t, err, nil)
	end, err := res.GetRow()
	checkErr(t, err, nil)
	if end != nil {
		t.Fatal(tmr)
	}

	if bytes.Compare(row[0].([]byte), big_blob) != 0 {
		t.Fatal("Full blob data don't match")
	}

	// Check second insert.
	id = 2
	res, err = sel.Run()
	checkErr(t, err, nil)

	row, err = res.GetRow()
	checkErr(t, err, nil)
	end, err = res.GetRow()
	checkErr(t, err, nil)
	if end != nil {
		t.Fatal(tmr)
	}

	if bytes.Compare(row.Bin(res.Map("bb")), data.Bb) != 0 {
		t.Fatal("Partial blob data don't match")
	}

	checkResult(t, query("drop table P"), cmdOK(0, false))
	myClose(t)
}

// Reconnect test

func TestReconnect(t *testing.T) {
	myConnect(t, true, 0)
	query("drop table R") // Drop test table if exists
	checkResult(t,
		query("create table R (id int primary key, str varchar(20))"),
		cmdOK(0, false),
	)

	ins, err := my.Prepare("insert R values (?, ?)")
	checkErr(t, err, nil)
	sel, err := my.Prepare("select str from R where id = ?")
	checkErr(t, err, nil)

	params := struct {
		Id  int
		Str string
	}{}
	var sel_id int

	ins.BindParams(&params)
	sel.BindParams(&sel_id)

	checkErr(t, my.Reconnect(), nil)

	params.Id = 1
	params.Str = "Bla bla bla"
	_, err = ins.Run()
	checkErr(t, err, nil)

	checkErr(t, my.Reconnect(), nil)

	sel_id = 1
	res, err := sel.Run()
	checkErr(t, err, nil)

	row, err := res.GetRow()
	checkErr(t, err, nil)

	checkErr(t, res.End(), nil)

	if row == nil || row[0] == nil ||
		params.Str != row.Str(0) {
		t.Fatal("Bad result")
	}

	checkErr(t, my.Reconnect(), nil)

	checkResult(t, query("drop table R"), cmdOK(0, false))
	myClose(t)
}

// StmtSendLongData test

func TestSendLongData(t *testing.T) {
	myConnect(t, true, 64*1024*1024)
	query("drop table L") // Drop test table if exists
	checkResult(t,
		query("create table L (id int primary key, bb longblob)"),
		cmdOK(0, false),
	)
	ins, err := my.Prepare("insert L values (?, ?)")
	checkErr(t, err, nil)

	sel, err := my.Prepare("select bb from L where id = ?")
	checkErr(t, err, nil)

	var (
		rre RowsResErr
		id  int64
	)

	ins.BindParams(&id, []byte(nil))
	sel.BindParams(&id)

	// Prepare data
	data := make([]byte, 4*1024*1024)
	for ii := range data {
		data[ii] = byte(ii)
	}
	// Send long data twice
	checkErr(t, ins.SendLongData(1, data, 256*1024), nil)
	checkErr(t, ins.SendLongData(1, data, 512*1024), nil)

	id = 1
	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	return
	res, err := sel.Run()
	checkErr(t, err, nil)

	row, err := res.GetRow()
	checkErr(t, err, nil)

	checkErr(t, res.End(), nil)

	if row == nil || row[0] == nil ||
		bytes.Compare(append(data, data...), row.Bin(0)) != 0 {
		t.Fatal("Bad result")
	}

	// Send long data from io.Reader twice
	filename := "_test/github.com/ziutek/mymysql/native.a"
	file, err := os.Open(filename)
	checkErr(t, err, nil)
	checkErr(t, ins.SendLongData(1, file, 128*1024), nil)
	checkErr(t, file.Close(), nil)
	file, err = os.Open(filename)
	checkErr(t, err, nil)
	checkErr(t, ins.SendLongData(1, file, 1024*1024), nil)
	checkErr(t, file.Close(), nil)

	id = 2
	rre.res, rre.err = ins.Run()
	checkResult(t, &rre, cmdOK(1, true))

	res, err = sel.Run()
	checkErr(t, err, nil)

	row, err = res.GetRow()
	checkErr(t, err, nil)

	checkErr(t, res.End(), nil)

	// Read file for check result
	data, err = ioutil.ReadFile(filename)
	checkErr(t, err, nil)

	if row == nil || row[0] == nil ||
		bytes.Compare(append(data, data...), row.Bin(0)) != 0 {
		t.Fatal("Bad result")
	}

	checkResult(t, query("drop table L"), cmdOK(0, false))
	myClose(t)
}

func TestMultipleResults(t *testing.T) {
	myConnect(t, true, 0)
	query("drop table M") // Drop test table if exists
	checkResult(t,
		query("create table M (id int primary key, str varchar(20))"),
		cmdOK(0, false),
	)

	str := []string{"zero", "jeden", "dwa"}

	checkResult(t, query("insert M values (0, '%s')", str[0]), cmdOK(1, false))
	checkResult(t, query("insert M values (1, '%s')", str[1]), cmdOK(1, false))
	checkResult(t, query("insert M values (2, '%s')", str[2]), cmdOK(1, false))

	res, err := my.Start("select id from M; select str from M")
	checkErr(t, err, nil)

	for ii := 0; ; ii++ {
		row, err := res.GetRow()
		checkErr(t, err, nil)
		if row == nil {
			break
		}
		if row.Int(0) != ii {
			t.Fatal("Bad result")
		}
	}
	res, err = res.NextResult()
	checkErr(t, err, nil)
	for ii := 0; ; ii++ {
		row, err := res.GetRow()
		checkErr(t, err, nil)
		if row == nil {
			break
		}
		if row.Str(0) != str[ii] {
			t.Fatal("Bad result")
		}
	}

	checkResult(t, query("drop table M"), cmdOK(0, false))
	myClose(t)
}

// Benchamrks

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func BenchmarkInsertSelect(b *testing.B) {
	b.StopTimer()

	my := New(conn[0], conn[1], conn[2], user, passwd, dbname)
	check(my.Connect())

	my.Start("drop table B") // Drop test table if exists

	_, err := my.Start("create table B (s varchar(40), i int)")
	check(err)

	for ii := 0; ii < 10000; ii++ {
		_, err := my.Start("insert B values ('%d-%d-%d', %d)", ii, ii, ii, ii)
		check(err)
	}

	b.StartTimer()

	for ii := 0; ii < b.N; ii++ {
		res, err := my.Start("select * from B")
		check(err)
		for {
			row, err := res.GetRow()
			check(err)
			if row == nil {
				break
			}
		}
	}

	b.StopTimer()

	_, err = my.Start("drop table B")
	check(err)
	check(my.Close())
}

func BenchmarkPreparedInsertSelect(b *testing.B) {
	b.StopTimer()

	my := New(conn[0], conn[1], conn[2], user, passwd, dbname)
	check(my.Connect())

	my.Start("drop table B") // Drop test table if exists

	_, err := my.Start("create table B (s varchar(40), i int)")
	check(err)

	ins, err := my.Prepare("insert B values (?, ?)")
	check(err)

	sel, err := my.Prepare("select * from B")
	check(err)

	for ii := 0; ii < 10000; ii++ {
		_, err := ins.Run(fmt.Sprintf("%d-%d-%d", ii, ii, ii), ii)
		check(err)
	}

	b.StartTimer()

	for ii := 0; ii < b.N; ii++ {
		res, err := sel.Run()
		check(err)
		for {
			row, err := res.GetRow()
			check(err)
			if row == nil {
				break
			}
		}
	}

	b.StopTimer()

	_, err = my.Start("drop table B")
	check(err)
	check(my.Close())
}