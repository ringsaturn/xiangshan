// Manually maintained; mirrors schema/xiangshan_remote.fbs RemoteIndex.

package v1

import flatbuffers "github.com/google/flatbuffers/go"

type RemoteIndex struct {
	_tab flatbuffers.Table
}

const RemoteIndexIdentifier = "XSRI"

func GetRootAsRemoteIndex(buf []byte, offset flatbuffers.UOffsetT) *RemoteIndex {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &RemoteIndex{}
	x.Init(buf, n+offset)
	return x
}

func RemoteIndexBufferHasIdentifier(buf []byte) bool {
	return flatbuffers.BufferHasIdentifier(buf, RemoteIndexIdentifier)
}

func GetSizePrefixedRootAsRemoteIndex(buf []byte, offset flatbuffers.UOffsetT) *RemoteIndex {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &RemoteIndex{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func FinishSizePrefixedRemoteIndexBuffer(builder *flatbuffers.Builder, offset flatbuffers.UOffsetT) {
	identifierBytes := []byte(RemoteIndexIdentifier)
	builder.FinishSizePrefixedWithFileIdentifier(offset, identifierBytes)
}

func SizePrefixedRemoteIndexBufferHasIdentifier(buf []byte) bool {
	return flatbuffers.SizePrefixedBufferHasIdentifier(buf, RemoteIndexIdentifier)
}

func (rcv *RemoteIndex) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *RemoteIndex) Table() flatbuffers.Table { return rcv._tab }

// slot 0 → vtable offset 4
func (rcv *RemoteIndex) Items(obj *RemoteDivisionMeta, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *RemoteIndex) ItemsLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

// slot 1 → vtable offset 6
func (rcv *RemoteIndex) GridCoarse(obj *GridIndex) *GridIndex {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(GridIndex)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

// slot 2 → vtable offset 8
func (rcv *RemoteIndex) GridFine(obj *GridIndex) *GridIndex {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(GridIndex)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

// slot 3 → vtable offset 10
func (rcv *RemoteIndex) CountryPreindex(obj *Preindex) *Preindex {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(Preindex)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

// slot 4 → vtable offset 12
func (rcv *RemoteIndex) Version() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 5 → vtable offset 14
func (rcv *RemoteIndex) Source() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// Builder functions

func RemoteIndexStart(builder *flatbuffers.Builder) { builder.StartObject(6) }

func RemoteIndexAddItems(builder *flatbuffers.Builder, items flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, items, 0)
}
func RemoteIndexStartItemsVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func RemoteIndexAddGridCoarse(builder *flatbuffers.Builder, gridCoarse flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, gridCoarse, 0)
}
func RemoteIndexAddGridFine(builder *flatbuffers.Builder, gridFine flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, gridFine, 0)
}
func RemoteIndexAddCountryPreindex(builder *flatbuffers.Builder, countryPreindex flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(3, countryPreindex, 0)
}
func RemoteIndexAddVersion(builder *flatbuffers.Builder, version flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(4, version, 0)
}
func RemoteIndexAddSource(builder *flatbuffers.Builder, source flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(5, source, 0)
}
func RemoteIndexEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
