// SPDX-License-Identifier: 0BSD
package source

// MediaSource produces encoded media bytes for an RNV session.
// Apps plug cameras, mics, files, or custom pipelines here.
type MediaSource interface {
	// Next returns the next encoded unit or an error when finished.
	Next() (payload []byte, err error)
	Close() error
}

// MediaSink consumes encoded media bytes from an RNV session.
type MediaSink interface {
	Write(payload []byte) error
	Close() error
}

// FuncSource adapts a function to MediaSource.
type FuncSource struct {
	Fn     func() ([]byte, error)
	Closer func() error
}

func (f *FuncSource) Next() ([]byte, error) {
	if f == nil || f.Fn == nil {
		return nil, errClosed
	}
	return f.Fn()
}

func (f *FuncSource) Close() error {
	if f != nil && f.Closer != nil {
		return f.Closer()
	}
	return nil
}

// FuncSink adapts a function to MediaSink.
type FuncSink struct {
	Fn     func([]byte) error
	Closer func() error
}

func (f *FuncSink) Write(payload []byte) error {
	if f == nil || f.Fn == nil {
		return errClosed
	}
	return f.Fn(payload)
}

func (f *FuncSink) Close() error {
	if f != nil && f.Closer != nil {
		return f.Closer()
	}
	return nil
}

type sourceError string

func (e sourceError) Error() string { return string(e) }

const errClosed = sourceError("rnv source: closed")
