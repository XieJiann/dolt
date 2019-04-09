package json

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/attic-labs/noms/go/types"
	"github.com/liquidata-inc/ld/dolt/go/cmd/dolt/cli"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/row"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/schema"
	"github.com/liquidata-inc/ld/dolt/go/libraries/utils/filesys"
	"github.com/liquidata-inc/ld/dolt/go/libraries/utils/iohelp"
)

const jsonHeader = `{"rows": [`
const jsonFooter = `]}`

var WriteBufSize = 256 * 1024

type JSONWriter struct {
	closer      io.Closer
	bWr         *bufio.Writer
	info        *JSONFileInfo
	sch         schema.Schema
	rowsWritten int
}

func OpenJSONWriter(path string, fs filesys.WritableFS, outSch schema.Schema, info *JSONFileInfo) (*JSONWriter, error) {
	err := fs.MkDirs(filepath.Dir(path))

	if err != nil {
		return nil, err
	}

	wr, err := fs.OpenForWrite(path)

	if err != nil {
		return nil, err
	}

	return NewJSONWriter(wr, outSch, info)
}

func NewJSONWriter(wr io.WriteCloser, outSch schema.Schema, info *JSONFileInfo) (*JSONWriter, error) {

	bwr := bufio.NewWriterSize(wr, WriteBufSize)
	err := iohelp.WriteAll(bwr, []byte(jsonHeader))
	if err != nil {
		return nil, err
	}
	return &JSONWriter{wr, bwr, info, outSch, 0}, nil
}

func (jsonw *JSONWriter) GetSchema() schema.Schema {
	return jsonw.sch
}

// WriteRow will write a row to a table
func (jsonw *JSONWriter) WriteRow(r row.Row) error {
	allCols := jsonw.sch.GetAllCols()
	colValMap := make(map[string]interface{}, allCols.Size())
	allCols.Iter(func(tag uint64, col schema.Column) (stop bool) {
		val, ok := r.GetColVal(tag)
		if ok && !types.IsNull(val) {
			colValMap[col.Name] = val
		}

		return false
	})

	data, err := marshalToJson(colValMap)
	if err != nil {
		return errors.New("marshaling did not work")
	}

	cli.Println(string(data))

	if jsonw.rowsWritten != 0 {
		jsonw.bWr.WriteRune(',')
	}

	newErr := iohelp.WriteAll(jsonw.bWr, data)
	if newErr != nil {
		return newErr
	}
	jsonw.rowsWritten++

	return nil
}

// Close should flush all writes, release resources being held
func (jsonw *JSONWriter) Close() error {
	if jsonw.closer != nil {
		err := iohelp.WriteAll(jsonw.bWr, []byte(jsonFooter))

		if err != nil {
			return err
		}

		errFl := jsonw.bWr.Flush()
		errCl := jsonw.closer.Close()
		jsonw.closer = nil

		if errCl != nil {
			return errCl
		}

		return errFl
	}
	return errors.New("already closed")

}

func marshalToJson(valMap interface{}) ([]byte, error) {
	var jsonBytes []byte
	var err error

	jsonBytes, err = json.Marshal(valMap)
	cli.Println(string(jsonBytes))
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}