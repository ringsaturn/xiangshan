// Manually maintained; mirrors schema/xiangshan_remote.fbs RemoteDivisionMeta.

package v1

import flatbuffers "github.com/google/flatbuffers/go"

// RemoteDivisionMeta is the lightweight per-division record stored in the
// remote index file. It holds all metadata needed for grid lookup and bbox
// pre-filtering, plus the slab offset/length for on-demand polygon fetching.
type RemoteDivisionMeta struct {
	_tab flatbuffers.Table
}

func GetRootAsRemoteDivisionMeta(buf []byte, offset flatbuffers.UOffsetT) *RemoteDivisionMeta {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &RemoteDivisionMeta{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsRemoteDivisionMeta(buf []byte, offset flatbuffers.UOffsetT) *RemoteDivisionMeta {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &RemoteDivisionMeta{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *RemoteDivisionMeta) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *RemoteDivisionMeta) Table() flatbuffers.Table { return rcv._tab }

// slot 0 → vtable offset 4
func (rcv *RemoteDivisionMeta) Id() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 1 → vtable offset 6
func (rcv *RemoteDivisionMeta) DivisionId() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 2 → vtable offset 8
func (rcv *RemoteDivisionMeta) Name() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 3 → vtable offset 10
func (rcv *RemoteDivisionMeta) Subtype() Subtype {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return Subtype(rcv._tab.GetInt8(o + rcv._tab.Pos))
	}
	return 0
}

// slot 4 → vtable offset 12
func (rcv *RemoteDivisionMeta) AdminLevel() int8 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.GetInt8(o + rcv._tab.Pos)
	}
	return 0
}

// slot 5 → vtable offset 14
func (rcv *RemoteDivisionMeta) Country() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 6 → vtable offset 16
func (rcv *RemoteDivisionMeta) Region() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 7 → vtable offset 18
func (rcv *RemoteDivisionMeta) ParentId() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(18))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 8 → vtable offset 20 (inline struct, not indirect)
func (rcv *RemoteDivisionMeta) Bbox(obj *BBox) *BBox {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(20))
	if o != 0 {
		x := o + rcv._tab.Pos
		if obj == nil {
			obj = new(BBox)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

// slot 9 → vtable offset 22
func (rcv *RemoteDivisionMeta) NamesCommon() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(22))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 10 → vtable offset 24
func (rcv *RemoteDivisionMeta) Class() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(24))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 11 → vtable offset 26
func (rcv *RemoteDivisionMeta) Wikidata() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(26))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 12 → vtable offset 28
func (rcv *RemoteDivisionMeta) Population() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(28))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

// slot 13 → vtable offset 30
func (rcv *RemoteDivisionMeta) DrivingSide() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(30))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 14 → vtable offset 32
func (rcv *RemoteDivisionMeta) LocalType() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(32))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

// slot 15 → vtable offset 34
func (rcv *RemoteDivisionMeta) PolyOffset() uint64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(34))
	if o != 0 {
		return rcv._tab.GetUint64(o + rcv._tab.Pos)
	}
	return 0
}

// slot 16 → vtable offset 36
func (rcv *RemoteDivisionMeta) PolyLength() uint32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(36))
	if o != 0 {
		return rcv._tab.GetUint32(o + rcv._tab.Pos)
	}
	return 0
}

// Builder functions

func RemoteDivisionMetaStart(builder *flatbuffers.Builder) { builder.StartObject(17) }

func RemoteDivisionMetaAddId(builder *flatbuffers.Builder, id flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, id, 0)
}
func RemoteDivisionMetaAddDivisionId(builder *flatbuffers.Builder, divisionId flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, divisionId, 0)
}
func RemoteDivisionMetaAddName(builder *flatbuffers.Builder, name flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, name, 0)
}
func RemoteDivisionMetaAddSubtype(builder *flatbuffers.Builder, subtype Subtype) {
	builder.PrependInt8Slot(3, int8(subtype), 0)
}
func RemoteDivisionMetaAddAdminLevel(builder *flatbuffers.Builder, adminLevel int8) {
	builder.PrependInt8Slot(4, adminLevel, 0)
}
func RemoteDivisionMetaAddCountry(builder *flatbuffers.Builder, country flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(5, country, 0)
}
func RemoteDivisionMetaAddRegion(builder *flatbuffers.Builder, region flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(6, region, 0)
}
func RemoteDivisionMetaAddParentId(builder *flatbuffers.Builder, parentId flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(7, parentId, 0)
}

// AddBbox must be called immediately after CreateBBox with no intervening writes.
func RemoteDivisionMetaAddBbox(builder *flatbuffers.Builder, bbox flatbuffers.UOffsetT) {
	builder.PrependStructSlot(8, flatbuffers.UOffsetT(bbox), 0)
}
func RemoteDivisionMetaAddNamesCommon(builder *flatbuffers.Builder, namesCommon flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(9, namesCommon, 0)
}
func RemoteDivisionMetaAddClass(builder *flatbuffers.Builder, class flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(10, class, 0)
}
func RemoteDivisionMetaAddWikidata(builder *flatbuffers.Builder, wikidata flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(11, wikidata, 0)
}
func RemoteDivisionMetaAddPopulation(builder *flatbuffers.Builder, population int32) {
	builder.PrependInt32Slot(12, population, 0)
}
func RemoteDivisionMetaAddDrivingSide(builder *flatbuffers.Builder, drivingSide flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(13, drivingSide, 0)
}
func RemoteDivisionMetaAddLocalType(builder *flatbuffers.Builder, localType flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(14, localType, 0)
}
func RemoteDivisionMetaAddPolyOffset(builder *flatbuffers.Builder, polyOffset uint64) {
	builder.PrependUint64Slot(15, polyOffset, 0)
}
func RemoteDivisionMetaAddPolyLength(builder *flatbuffers.Builder, polyLength uint32) {
	builder.PrependUint32Slot(16, polyLength, 0)
}

func RemoteDivisionMetaEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
