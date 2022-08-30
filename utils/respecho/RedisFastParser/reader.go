package RedisFastParser

import (
	"io"
)

var (
	bufStepSize = 1024
)

type Reader struct {
	reader        io.Reader
	Buffer        []byte
	ReadPosition  int //解析器读取位置,类似缓冲区读地址
	WritePosition int //缓冲区写地址
}

func NewReader(reader io.Reader) *Reader {
	return &Reader{reader: reader, Buffer: make([]byte, bufStepSize)}
}

// 判断是否还有充足的所需空间
func (r *Reader) requestSpace(reqSize int) {
	ccap := cap(r.Buffer)
	if r.WritePosition+reqSize > ccap {
		newbuff := make([]byte, max(ccap*2, ccap+reqSize+bufStepSize))
		copy(newbuff, r.Buffer)
		r.Buffer = newbuff
	}
}

// 继续读取部分数据 min为最小读取字节
func (r *Reader) ReadSome(min int) error {
	r.requestSpace(min) //检测并开辟所需的必要存储单元
	nr, err := io.ReadAtLeast(r.reader, r.Buffer[r.WritePosition:], min)
	if err != nil {
		return err
	}
	r.WritePosition += nr

	return nil
}

//检验是否有num byte的数据,如果没有,则把差值从网络中读取到,此处的读取会阻塞
func (r *Reader) RequireNBytes(num int) error {
	a := r.WritePosition - r.ReadPosition
	if a >= num {
		return nil
	}
	if err := r.ReadSome(num - a); err != nil {
		return err
	}
	return nil
}

//从缓冲区中获取指定长度的byte数据 并修改指针
func (r *Reader) GetNbytes(num int) (data []byte, err error) {

	err = r.RequireNBytes(num)

	if err != nil {
		return
	}

	data = r.Buffer[r.ReadPosition : r.ReadPosition+num]

	r.ReadPosition += num
	return
}

//判断是否已经结束读取操作
func (r *Reader) IsEnd() (ret bool) {

	ret = false

	if r.ReadPosition >= r.WritePosition {
		ret = true
		r.Reset()
	}

	return
}

func (r *Reader) Reset() {
	r.WritePosition = 0
	r.ReadPosition = 0
}
