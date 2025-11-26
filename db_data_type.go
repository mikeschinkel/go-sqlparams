package sqlparams

import (
	"strings"
)

type DBAccessMode int

const (
	UnspecifiedDBAccessMode DBAccessMode = iota
	DBReadOnlyMode
	DBReadWriteMode
	DBAdminMode
	DBSuperAdminMode
)

type DBDataTypes []DBDataType
type DBDataType string

func (dt DBDataType) Normalize() DBDataType {
	switch dt {
	case IntDBDataType:
		return IntegerDBDataType
	case IntDBDataTypeOrNULL:
		return IntegerDBDataTypeOrNULL
	}
	return dt
}

const (
	AnyDBDataType           DBDataType = "any"
	IntegerDBDataType       DBDataType = "integer"
	IntDBDataType           DBDataType = "int"
	RealDBDataType          DBDataType = "real"
	StringDBDataType        DBDataType = "string"
	JSONDBDataType          DBDataType = "json"
	IntegerDBDataTypeOrNULL DBDataType = "integer?"
	IntDBDataTypeOrNULL     DBDataType = "int?"
	RealDBDataTypeOrNULL    DBDataType = "real?"
	StringDBDataTypeOrNULL  DBDataType = "string?"
	JSONDBDataTypeOrNULL    DBDataType = "json?"
)

type DBRowType string

func (rt DBRowType) Normalize() DBRowType {
	switch rt {
	case IntRowType:
		return IntegerRowType
	case IntRowTypeOrNULL:
		return IntegerRowTypeOrNULL
	}
	return rt
}

const (
	AnyRowType           DBRowType = "any"
	IntegerRowType       DBRowType = "integer"
	IntRowType           DBRowType = "int"
	RealRowType          DBRowType = "real"
	StringRowType        DBRowType = "string"
	ColumnsRowType       DBRowType = "columns"
	JSONRowType          DBRowType = "json"
	IntegerRowTypeOrNULL DBRowType = "integer?"
	IntRowTypeOrNULL     DBRowType = "int?"
	RealRowTypeOrNULL    DBRowType = "real?"
	StringRowTypeOrNULL  DBRowType = "string?"
	JSONRowTypeOrNULL    DBRowType = "json?"
)

func ParseDBRowType(s string) (rt DBRowType, err error) {
	if s == "" {
		rt = DefaultRowType
		goto end
	}
	rt = DBRowType(strings.ToLower(s)).Normalize()
	switch rt {
	case AnyRowType, IntegerRowType, RealRowType, StringRowType, ColumnsRowType, JSONRowType, IntegerRowTypeOrNULL, RealRowTypeOrNULL, StringRowTypeOrNULL, JSONRowTypeOrNULL:
		// Nothing to do
	default:
		err = NewErr(ErrInvalidRowType, "row_type", s)
		rt = ""
	}
end:
	return rt, err
}

func ParseColumnTypes(ss []string) (cts []DBDataType, err error) {
	var errs []error
	if len(ss) == 0 {
		goto end
	}
	cts = make([]DBDataType, len(ss))
	for i, s := range ss {
		value, err := ParseDBDataType(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cts[i] = value
	}
	err = CombineErrs(errs)
	if err != nil {
		err = NewErr(ErrInvalidResultsColumnDataType, err)
	}
end:
	return cts, err
}

func ParseDBDataType(s string) (dt DBDataType, err error) {
	if s == "" {
		dt = DefaultDBDataType
		goto end
	}
	dt = DBDataType(strings.ToLower(s)).Normalize()
	switch dt {
	case AnyDBDataType, IntegerDBDataType, RealDBDataType, StringDBDataType, JSONDBDataType, IntegerDBDataTypeOrNULL, RealDBDataTypeOrNULL, StringDBDataTypeOrNULL, JSONDBDataTypeOrNULL:
		// Nothing to do
	default:
		err = NewErr(ErrInvalidDataType, "data_type", s)
		dt = ""
	}
end:
	return dt, err
}
