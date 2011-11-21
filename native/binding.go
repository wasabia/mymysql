package native

import (
    "reflect"
	"github.com/ziutek/mymysql"
)

var (
    reflectBlobType = reflect.TypeOf(mysql.Blob{})
    reflectDatetimeType = reflect.TypeOf(mysql.Datetime{})
    reflectDateType = reflect.TypeOf(mysql.Date{})
    reflectTimestampType = reflect.TypeOf(mysql.Timestamp{})
    reflectTimeType = reflect.TypeOf(mysql.Time(0))
    reflectRawType = reflect.TypeOf(mysql.Raw{})
)

// val should be an addressable value
func bindValue(val reflect.Value) (out *paramValue) {
    if !val.IsValid() {
        return &paramValue{typ: MYSQL_TYPE_NULL}
    }
    typ := val.Type()
	out = new(paramValue)
    if typ.Kind() == reflect.Ptr {
        // We have addressable pointer
        out.SetAddr(val.UnsafeAddr())
        // Dereference pointer for next operation on its value
        typ = typ.Elem()
        val = val.Elem()
    } else {
        // We have addressable value. Create a pointer to it
        pv := val.Addr()
        // This pointer is unaddressable so copy it and return an address
        ppv := reflect.New(pv.Type())
        ppv.Elem().Set(pv)
        out.SetAddr(ppv.Pointer())
    }

    // Obtain value type
    switch typ.Kind() {
    case reflect.String:
        out.typ    = MYSQL_TYPE_STRING
        out.length = -1
        return

    case reflect.Int:
        out.typ = _INT_TYPE
        out.length = _SIZE_OF_INT
        return

    case reflect.Int8:
        out.typ = MYSQL_TYPE_TINY
        out.length = 1
        return

    case reflect.Int16:
        out.typ = MYSQL_TYPE_SHORT
        out.length = 2
        return

    case reflect.Int32:
        out.typ = MYSQL_TYPE_LONG
        out.length = 4
        return

    case reflect.Int64:
        if typ == reflectTimeType {
            out.typ = MYSQL_TYPE_TIME
            out.length = -1
            return
        }
        out.typ = MYSQL_TYPE_LONGLONG
        out.length = 8
        return

    case reflect.Uint:
        out.typ = _INT_TYPE | MYSQL_UNSIGNED_MASK
        out.length = _SIZE_OF_INT
        return

    case reflect.Uint8:
        out.typ = MYSQL_TYPE_TINY | MYSQL_UNSIGNED_MASK
        out.length = 1
        return

    case reflect.Uint16:
        out.typ = MYSQL_TYPE_SHORT | MYSQL_UNSIGNED_MASK
        out.length = 2
        return

    case reflect.Uint32:
        out.typ = MYSQL_TYPE_LONG | MYSQL_UNSIGNED_MASK
        out.length = 4
        return

    case reflect.Uint64:
        if typ == reflectTimeType {
            out.typ = MYSQL_TYPE_TIME
            out.length = -1
            return
        }
        out.typ = MYSQL_TYPE_LONGLONG | MYSQL_UNSIGNED_MASK
        out.length = 8
        return

    case reflect.Float32:
        out.typ = MYSQL_TYPE_FLOAT
        out.length = 4
        return

    case reflect.Float64:
        out.typ = MYSQL_TYPE_DOUBLE
        out.length = 8
        return

    case reflect.Slice:
        out.length = -1
        if typ == reflectBlobType {
            out.typ = MYSQL_TYPE_BLOB
            return
        }
        if typ.Elem().Kind() == reflect.Uint8 {
            out.typ = MYSQL_TYPE_VAR_STRING
            return
        }

    case reflect.Struct:
        out.length = -1
        if typ == reflectDatetimeType {
            out.typ = MYSQL_TYPE_DATETIME
            return
        }
        if typ == reflectDateType {
            out.typ = MYSQL_TYPE_DATE
            return
        }
        if typ == reflectTimestampType {
            out.typ = MYSQL_TYPE_TIMESTAMP
            return
        }
        if typ == reflectRawType {
            out.typ = val.FieldByName("Typ").Interface().(uint16)
            out.SetAddr(val.FieldByName("Val").Pointer())
            out.raw = true
            return
        }
    }
    panic(BIND_UNK_TYPE)
}