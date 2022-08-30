package RedSHandle

import "io"

type Handle struct {
	Parser *ParserHandle
	Writer *WriterHandle
}

func NewHandle(reader io.Reader, writer io.Writer) (*Handle ) {

	//NewParserHandle(reader)
	return &Handle{NewParserHandle(reader), NewWriterHandle(writer)}
}

