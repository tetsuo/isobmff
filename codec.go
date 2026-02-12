package mp4

import (
	"encoding/binary"
	"fmt"
)

// Codec types for each known box.

// Ftyp represents the file type box.
type Ftyp struct {
	Brand            [4]byte
	BrandVersion     uint32
	CompatibleBrands [][4]byte
}

// Mvhd represents the movie header box.
type Mvhd struct {
	CTime             [4]byte // raw 4-byte date
	MTime             [4]byte
	TimeScale         uint32
	Duration          uint32
	PreferredRate     [4]byte // raw 16.16 fixed
	PreferredVolume   [2]byte // raw 8.8 fixed
	Matrix            [36]byte
	PreviewTime       uint32
	PreviewDuration   uint32
	PosterTime        uint32
	SelectionTime     uint32
	SelectionDuration uint32
	CurrentTime       uint32
	NextTrackId       uint32
}

// Tkhd represents the track header box.
type Tkhd struct {
	CTime          [4]byte
	MTime          [4]byte
	TrackId        uint32
	Duration       uint32
	Layer          uint16
	AlternateGroup uint16
	Volume         uint16
	Matrix         [36]byte
	TrackWidth     uint32
	TrackHeight    uint32
}

// Mdhd represents the media header box.
type Mdhd struct {
	CTime     [8]byte // 4 or 8 bytes depending on version
	MTime     [8]byte
	TimeScale uint32
	Duration  uint64
	Language  uint16
	Quality   uint16
	V1        bool // true if version 1
}

// Vmhd represents the video media header box.
type Vmhd struct {
	GraphicsMode uint16
	Opcolor      [3]uint16
}

// Smhd represents the sound media header box.
type Smhd struct {
	Balance uint16
}

// Stsd represents the sample description box.
type Stsd struct {
	Entries []*Box
}

// VisualSampleEntry represents a visual sample entry (e.g. avc1).
type VisualSampleEntry struct {
	DataReferenceIndex uint16
	Width              uint16
	Height             uint16
	HResolution        uint32
	VResolution        uint32
	FrameCount         uint16
	CompressorName     string
	Depth              uint16
	Children           []*Box
}

// AvcC represents the AVC configuration box.
type AvcC struct {
	MimeCodec string
	Buffer    []byte
}

// AudioSampleEntry represents an audio sample entry (e.g. mp4a).
type AudioSampleEntry struct {
	DataReferenceIndex uint16
	ChannelCount       uint16
	SampleSize         uint16
	SampleRate         uint32
	Children           []*Box
}

// Esds represents the elementary stream descriptor box.
type Esds struct {
	MimeCodec string
	Buffer    []byte
}

// Stsz represents the sample size box.
type Stsz struct {
	SampleSize uint32
	Entries    []uint32
}

// Stco represents the chunk offset box (also used for stss).
type Stco struct {
	Entries []uint32
}

// Co64 represents the 64-bit chunk offset box.
type Co64 struct {
	Entries []uint64
}

// STTSEntry is a time-to-sample entry.
type STTSEntry struct {
	Count    uint32
	Duration uint32
}

// Stts represents the time-to-sample box.
type Stts struct {
	Entries []STTSEntry
}

// CTTSEntry is a composition offset entry.
type CTTSEntry struct {
	Count             uint32
	CompositionOffset int32
}

// Ctts represents the composition offset box.
type Ctts struct {
	Entries []CTTSEntry
}

// STSCEntry is a sample-to-chunk entry.
type STSCEntry struct {
	FirstChunk          uint32
	SamplesPerChunk     uint32
	SampleDescriptionId uint32
}

// Stsc represents the sample-to-chunk box.
type Stsc struct {
	Entries []STSCEntry
}

// DrefEntry is a data reference entry.
type DrefEntry struct {
	Type [4]byte
	Buf  []byte
}

// DrefBox represents the data reference box.
type DrefBox struct {
	Entries []DrefEntry
}

// ElstEntry is an edit list entry.
type ElstEntry struct {
	TrackDuration uint32
	MediaTime     int32
	MediaRate     [4]byte // raw fixed 16.16
}

// Elst represents the edit list box.
type Elst struct {
	Entries []ElstEntry
}

// Hdlr represents the handler reference box.
type Hdlr struct {
	HandlerType [4]byte
	Name        string
}

// Mehd represents the movie extends header box.
type Mehd struct {
	FragmentDuration uint32
}

// Trex represents the track extends box.
type Trex struct {
	TrackId                       uint32
	DefaultSampleDescriptionIndex uint32
	DefaultSampleDuration         uint32
	DefaultSampleSize             uint32
	DefaultSampleFlags            uint32
}

// Mfhd represents the movie fragment header box.
type Mfhd struct {
	SequenceNumber uint32
}

// Tfhd represents the track fragment header box.
type Tfhd struct {
	TrackId uint32
}

// Tfdt represents the track fragment decode time box.
type Tfdt struct {
	BaseMediaDecodeTime uint32
}

// TrunEntry is a track run entry.
type TrunEntry struct {
	SampleDuration              uint32
	SampleSize                  uint32
	SampleFlags                 uint32
	SampleCompositionTimeOffset int32
}

// Trun represents the track run box.
type Trun struct {
	DataOffset int32
	Entries    []TrunEntry
}

// Mdat represents the media data box.
type Mdat struct {
	Buffer        []byte
	ContentLength int
}

// codec is an internal interface for box-specific encoding/decoding.
type codec struct {
	decode         func(box *Box, buf []byte, start, end int) error
	encode         func(box *Box, buf []byte, offset int) int
	encodingLength func(box *Box) int
}

var codecs = map[BoxType]*codec{}

func getCodec(t BoxType) *codec {
	return codecs[t]
}

func init() {
	codecs[TypeFtyp] = &codec{decodeFtyp, encodeFtyp, encodingLengthFtyp}
	codecs[TypeMvhd] = &codec{decodeMvhd, encodeMvhd, encodingLengthMvhd}
	codecs[TypeTkhd] = &codec{decodeTkhd, encodeTkhd, encodingLengthTkhd}
	codecs[TypeMdhd] = &codec{decodeMdhd, encodeMdhd, encodingLengthMdhd}
	codecs[TypeVmhd] = &codec{decodeVmhd, encodeVmhd, encodingLengthVmhd}
	codecs[TypeSmhd] = &codec{decodeSmhd, encodeSmhd, encodingLengthSmhd}
	codecs[TypeStsd] = &codec{decodeStsd, encodeStsd, encodingLengthStsd}
	codecs[TypeAvc1] = &codec{decodeVisual, encodeVisual, encodingLengthVisual}
	codecs[TypeAvcC] = &codec{decodeAvcC, encodeAvcC, encodingLengthAvcC}
	codecs[TypeMp4a] = &codec{decodeAudio, encodeAudio, encodingLengthAudio}
	codecs[TypeEsds] = &codec{decodeEsds, encodeEsds, encodingLengthEsds}
	codecs[TypeStsz] = &codec{decodeStsz, encodeStsz, encodingLengthStsz}
	codecs[TypeStco] = &codec{decodeStco, encodeStco, encodingLengthStco}
	codecs[TypeStss] = &codec{decodeStco, encodeStco, encodingLengthStco} // same format as stco
	codecs[TypeCo64] = &codec{decodeCo64, encodeCo64, encodingLengthCo64}
	codecs[TypeStts] = &codec{decodeStts, encodeStts, encodingLengthStts}
	codecs[TypeCtts] = &codec{decodeCtts, encodeCtts, encodingLengthCtts}
	codecs[TypeStsc] = &codec{decodeStsc, encodeStsc, encodingLengthStsc}
	codecs[TypeDref] = &codec{decodeDref, encodeDref, encodingLengthDref}
	codecs[TypeElst] = &codec{decodeElst, encodeElst, encodingLengthElst}
	codecs[TypeHdlr] = &codec{decodeHdlr, encodeHdlr, encodingLengthHdlr}
	codecs[TypeMehd] = &codec{decodeMehd, encodeMehd, encodingLengthMehd}
	codecs[TypeTrex] = &codec{decodeTrex, encodeTrex, encodingLengthTrex}
	codecs[TypeMfhd] = &codec{decodeMfhd, encodeMfhd, encodingLengthMfhd}
	codecs[TypeTfhd] = &codec{decodeTfhd, encodeTfhd, encodingLengthTfhd}
	codecs[TypeTfdt] = &codec{decodeTfdt, encodeTfdt, encodingLengthTfdt}
	codecs[TypeTrun] = &codec{decodeTrun, encodeTrun, encodingLengthTrun}
	codecs[TypeMdat] = &codec{decodeMdat, encodeMdat, encodingLengthMdat}
}

// --- ftyp ---

func decodeFtyp(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	if len(b) < 8 {
		return fmt.Errorf("ftyp too short")
	}
	f := &Ftyp{}
	copy(f.Brand[:], b[0:4])
	f.BrandVersion = be.Uint32(b[4:8])
	for i := 8; i+4 <= len(b); i += 4 {
		var brand [4]byte
		copy(brand[:], b[i:i+4])
		f.CompatibleBrands = append(f.CompatibleBrands, brand)
	}
	box.Ftyp = f
	return nil
}

func encodeFtyp(box *Box, buf []byte, offset int) int {
	f := box.Ftyp
	b := buf[offset:]
	copy(b[0:4], f.Brand[:])
	be.PutUint32(b[4:8], f.BrandVersion)
	for i, brand := range f.CompatibleBrands {
		copy(b[8+i*4:], brand[:])
	}
	return 8 + len(f.CompatibleBrands)*4
}

func encodingLengthFtyp(box *Box) int {
	return 8 + len(box.Ftyp.CompatibleBrands)*4
}

// --- mvhd ---

func decodeMvhd(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	m := &Mvhd{}
	copy(m.CTime[:], b[0:4])
	copy(m.MTime[:], b[4:8])
	m.TimeScale = be.Uint32(b[8:12])
	m.Duration = be.Uint32(b[12:16])
	copy(m.PreferredRate[:], b[16:20])
	copy(m.PreferredVolume[:], b[20:22])
	copy(m.Matrix[:], b[32:68])
	m.PreviewTime = be.Uint32(b[68:72])
	m.PreviewDuration = be.Uint32(b[72:76])
	m.PosterTime = be.Uint32(b[76:80])
	m.SelectionTime = be.Uint32(b[80:84])
	m.SelectionDuration = be.Uint32(b[84:88])
	m.CurrentTime = be.Uint32(b[88:92])
	m.NextTrackId = be.Uint32(b[92:96])
	box.Mvhd = m
	return nil
}

func encodeMvhd(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	m := box.Mvhd
	copy(b[0:4], m.CTime[:])
	copy(b[4:8], m.MTime[:])
	be.PutUint32(b[8:12], m.TimeScale)
	be.PutUint32(b[12:16], m.Duration)
	copy(b[16:20], m.PreferredRate[:])
	copy(b[20:22], m.PreferredVolume[:])
	clearBytes(b, 22, 32)
	copy(b[32:68], m.Matrix[:])
	be.PutUint32(b[68:72], m.PreviewTime)
	be.PutUint32(b[72:76], m.PreviewDuration)
	be.PutUint32(b[76:80], m.PosterTime)
	be.PutUint32(b[80:84], m.SelectionTime)
	be.PutUint32(b[84:88], m.SelectionDuration)
	be.PutUint32(b[88:92], m.CurrentTime)
	be.PutUint32(b[92:96], m.NextTrackId)
	return 96
}

func encodingLengthMvhd(_ *Box) int { return 96 }

// --- tkhd ---

func decodeTkhd(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	t := &Tkhd{}
	copy(t.CTime[:], b[0:4])
	copy(t.MTime[:], b[4:8])
	t.TrackId = be.Uint32(b[8:12])
	t.Duration = be.Uint32(b[16:20])
	t.Layer = be.Uint16(b[28:30])
	t.AlternateGroup = be.Uint16(b[30:32])
	t.Volume = be.Uint16(b[32:34])
	copy(t.Matrix[:], b[36:72])
	t.TrackWidth = be.Uint32(b[72:76])
	t.TrackHeight = be.Uint32(b[76:80])
	box.Tkhd = t
	return nil
}

func encodeTkhd(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	t := box.Tkhd
	copy(b[0:4], t.CTime[:])
	copy(b[4:8], t.MTime[:])
	be.PutUint32(b[8:12], t.TrackId)
	clearBytes(b, 12, 16)
	be.PutUint32(b[16:20], t.Duration)
	clearBytes(b, 20, 28)
	be.PutUint16(b[28:30], t.Layer)
	be.PutUint16(b[30:32], t.AlternateGroup)
	be.PutUint16(b[32:34], t.Volume)
	clearBytes(b, 34, 36)
	copy(b[36:72], t.Matrix[:])
	be.PutUint32(b[72:76], t.TrackWidth)
	be.PutUint32(b[76:80], t.TrackHeight)
	return 80
}

func encodingLengthTkhd(_ *Box) int { return 80 }

// --- mdhd ---

func decodeMdhd(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	m := &Mdhd{}
	contentLen := end - start

	if contentLen != 20 {
		// version 1: 8-byte times
		m.V1 = true
		copy(m.CTime[:], b[0:8])
		copy(m.MTime[:], b[8:16])
		m.TimeScale = be.Uint32(b[16:20])
		// Read 6 bytes for duration (48-bit)
		m.Duration = uint64(b[20])<<40 | uint64(b[21])<<32 | uint64(b[22])<<24 |
			uint64(b[23])<<16 | uint64(b[24])<<8 | uint64(b[25])
		m.Language = be.Uint16(b[28:30])
		m.Quality = be.Uint16(b[30:32])
	} else {
		copy(m.CTime[:4], b[0:4])
		copy(m.MTime[:4], b[4:8])
		m.TimeScale = be.Uint32(b[8:12])
		m.Duration = uint64(be.Uint32(b[12:16]))
		m.Language = be.Uint16(b[16:18])
		m.Quality = be.Uint16(b[18:20])
	}
	box.Mdhd = m
	return nil
}

func encodeMdhd(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	m := box.Mdhd

	if m.V1 {
		copy(b[0:8], m.CTime[:])
		copy(b[8:16], m.MTime[:])
		be.PutUint32(b[16:20], m.TimeScale)
		// Write 6 bytes for duration
		b[20] = byte(m.Duration >> 40)
		b[21] = byte(m.Duration >> 32)
		b[22] = byte(m.Duration >> 24)
		b[23] = byte(m.Duration >> 16)
		b[24] = byte(m.Duration >> 8)
		b[25] = byte(m.Duration)
		clearBytes(b, 26, 28)
		be.PutUint16(b[28:30], m.Language)
		be.PutUint16(b[30:32], m.Quality)
		return 32
	}

	copy(b[0:4], m.CTime[:4])
	copy(b[4:8], m.MTime[:4])
	be.PutUint32(b[8:12], m.TimeScale)
	be.PutUint32(b[12:16], uint32(m.Duration))
	be.PutUint16(b[16:18], m.Language)
	be.PutUint16(b[18:20], m.Quality)
	return 20
}

func encodingLengthMdhd(box *Box) int {
	if box.Mdhd.V1 {
		return 32
	}
	return 20
}

// --- vmhd ---

func decodeVmhd(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	box.Vmhd = &Vmhd{
		GraphicsMode: be.Uint16(b[0:2]),
		Opcolor:      [3]uint16{be.Uint16(b[2:4]), be.Uint16(b[4:6]), be.Uint16(b[6:8])},
	}
	return nil
}

func encodeVmhd(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	v := box.Vmhd
	be.PutUint16(b[0:2], v.GraphicsMode)
	be.PutUint16(b[2:4], v.Opcolor[0])
	be.PutUint16(b[4:6], v.Opcolor[1])
	be.PutUint16(b[6:8], v.Opcolor[2])
	return 8
}

func encodingLengthVmhd(_ *Box) int { return 8 }

// --- smhd ---

func decodeSmhd(box *Box, buf []byte, start, _ int) error {
	box.Smhd = &Smhd{Balance: be.Uint16(buf[start:])}
	return nil
}

func encodeSmhd(box *Box, buf []byte, offset int) int {
	be.PutUint16(buf[offset:], box.Smhd.Balance)
	clearBytes(buf, offset+2, offset+4)
	return 4
}

func encodingLengthSmhd(_ *Box) int { return 4 }

// --- stsd ---

func decodeStsd(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	num := int(be.Uint32(b[0:4]))
	s := &Stsd{Entries: make([]*Box, num)}
	ptr := 4
	for i := 0; i < num; i++ {
		entry, err := Decode(buf, start+ptr, end)
		if err != nil {
			return err
		}
		s.Entries[i] = entry
		ptr += int(entry.Size)
	}
	box.Stsd = s
	return nil
}

func encodeStsd(box *Box, buf []byte, offset int) int {
	s := box.Stsd
	be.PutUint32(buf[offset:], uint32(len(s.Entries)))
	ptr := offset + 4
	for _, entry := range s.Entries {
		n, _ := encodeBox(entry, buf, ptr)
		ptr += n
	}
	return ptr - offset
}

func encodingLengthStsd(box *Box) int {
	total := 4
	for _, entry := range box.Stsd.Entries {
		total += int(EncodingLength(entry))
	}
	return total
}

// --- avc1 / VisualSampleEntry ---

func decodeVisual(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	length := end - start
	nameLen := int(b[42])
	if nameLen > 31 {
		nameLen = 31
	}
	v := &VisualSampleEntry{
		DataReferenceIndex: be.Uint16(b[6:8]),
		Width:              be.Uint16(b[24:26]),
		Height:             be.Uint16(b[26:28]),
		HResolution:        be.Uint32(b[28:32]),
		VResolution:        be.Uint32(b[32:36]),
		FrameCount:         be.Uint16(b[40:42]),
		CompressorName:     string(b[43 : 43+nameLen]),
		Depth:              be.Uint16(b[74:76]),
	}

	ptr := 78
	for length-ptr >= 8 {
		child, err := Decode(buf, start+ptr, end)
		if err != nil {
			return err
		}
		v.Children = append(v.Children, child)
		ptr += int(child.Size)
	}
	box.Visual = v
	return nil
}

func encodeVisual(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	v := box.Visual
	clearBytes(b, 0, 6)
	be.PutUint16(b[6:8], v.DataReferenceIndex)
	clearBytes(b, 8, 24)
	be.PutUint16(b[24:26], v.Width)
	be.PutUint16(b[26:28], v.Height)
	hRes := v.HResolution
	if hRes == 0 {
		hRes = 0x480000
	}
	be.PutUint32(b[28:32], hRes)
	vRes := v.VResolution
	if vRes == 0 {
		vRes = 0x480000
	}
	be.PutUint32(b[32:36], vRes)
	clearBytes(b, 36, 40)
	fc := v.FrameCount
	if fc == 0 {
		fc = 1
	}
	be.PutUint16(b[40:42], fc)
	nameLen := len(v.CompressorName)
	if nameLen > 31 {
		nameLen = 31
	}
	b[42] = byte(nameLen)
	copy(b[43:], v.CompressorName[:nameLen])
	clearBytes(b, 43+nameLen, 74)
	depth := v.Depth
	if depth == 0 {
		depth = 0x18
	}
	be.PutUint16(b[74:76], depth)
	binary.BigEndian.PutUint16(b[76:78], 0xffff) // -1 as int16

	ptr := 78
	for _, child := range v.Children {
		n, _ := encodeBox(child, buf, offset+ptr)
		ptr += n
	}
	return ptr
}

func encodingLengthVisual(box *Box) int {
	n := 78
	for _, child := range box.Visual.Children {
		n += int(EncodingLength(child))
	}
	return n
}

// --- avcC ---

func decodeAvcC(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	a := &AvcC{
		Buffer: make([]byte, len(b)),
	}
	copy(a.Buffer, b)
	if len(b) >= 4 {
		a.MimeCodec = fmt.Sprintf("%02x%02x%02x", b[1], b[2], b[3])
	}
	box.AvcC = a
	return nil
}

func encodeAvcC(box *Box, buf []byte, offset int) int {
	copy(buf[offset:], box.AvcC.Buffer)
	return len(box.AvcC.Buffer)
}

func encodingLengthAvcC(box *Box) int { return len(box.AvcC.Buffer) }

// --- mp4a / AudioSampleEntry ---

func decodeAudio(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	length := end - start
	a := &AudioSampleEntry{
		DataReferenceIndex: be.Uint16(b[6:8]),
		ChannelCount:       be.Uint16(b[16:18]),
		SampleSize:         be.Uint16(b[18:20]),
		SampleRate:         be.Uint32(b[24:28]),
	}

	ptr := 28
	for length-ptr >= 8 {
		child, err := Decode(buf, start+ptr, end)
		if err != nil {
			return err
		}
		a.Children = append(a.Children, child)
		ptr += int(child.Size)
	}
	box.Audio = a
	return nil
}

func encodeAudio(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	a := box.Audio
	clearBytes(b, 0, 6)
	be.PutUint16(b[6:8], a.DataReferenceIndex)
	clearBytes(b, 8, 16)
	cc := a.ChannelCount
	if cc == 0 {
		cc = 2
	}
	be.PutUint16(b[16:18], cc)
	ss := a.SampleSize
	if ss == 0 {
		ss = 16
	}
	be.PutUint16(b[18:20], ss)
	clearBytes(b, 20, 24)
	be.PutUint32(b[24:28], a.SampleRate)

	ptr := 28
	for _, child := range a.Children {
		n, _ := encodeBox(child, buf, offset+ptr)
		ptr += n
	}
	return ptr
}

func encodingLengthAudio(box *Box) int {
	n := 28
	for _, child := range box.Audio.Children {
		n += int(EncodingLength(child))
	}
	return n
}

// --- esds ---

func decodeEsds(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	e := &Esds{
		Buffer: make([]byte, len(b)),
	}
	copy(e.Buffer, b)

	// Parse descriptor for mimeCodec
	desc := decodeDescriptor(b, 0, len(b))
	if desc != nil && desc.tagName == "ESDescriptor" {
		if dcd, ok := desc.children["DecoderConfigDescriptor"]; ok {
			oti := dcd.oti
			if oti != 0 {
				e.MimeCodec = fmt.Sprintf("%x", oti)
				if dsi, ok := dcd.children["DecoderSpecificInfo"]; ok && len(dsi.buffer) > 0 {
					audioConfig := (dsi.buffer[0] & 0xf8) >> 3
					if audioConfig != 0 {
						e.MimeCodec += fmt.Sprintf(".%d", audioConfig)
					}
				}
			}
		}
	}
	box.Esds = e
	return nil
}

func encodeEsds(box *Box, buf []byte, offset int) int {
	copy(buf[offset:], box.Esds.Buffer)
	return len(box.Esds.Buffer)
}

func encodingLengthEsds(box *Box) int { return len(box.Esds.Buffer) }

// --- stsz ---

func decodeStsz(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	sampleSize := be.Uint32(b[0:4])
	num := int(be.Uint32(b[4:8]))
	entries := make([]uint32, num)
	for i := 0; i < num; i++ {
		if sampleSize == 0 {
			entries[i] = be.Uint32(b[8+i*4:])
		} else {
			entries[i] = sampleSize
		}
	}
	box.Stsz = &Stsz{SampleSize: sampleSize, Entries: entries}
	return nil
}

func encodeStsz(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Stsz
	be.PutUint32(b[0:4], 0)
	be.PutUint32(b[4:8], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		be.PutUint32(b[8+i*4:], e)
	}
	return 8 + len(s.Entries)*4
}

func encodingLengthStsz(box *Box) int {
	return 8 + len(box.Stsz.Entries)*4
}

// --- stco / stss ---

func decodeStco(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]uint32, num)
	for i := 0; i < num; i++ {
		entries[i] = be.Uint32(b[4+i*4:])
	}
	box.Stco = &Stco{Entries: entries}
	return nil
}

func encodeStco(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Stco
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		be.PutUint32(b[4+i*4:], e)
	}
	return 4 + len(s.Entries)*4
}

func encodingLengthStco(box *Box) int {
	return 4 + len(box.Stco.Entries)*4
}

// --- co64 ---

func decodeCo64(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]uint64, num)
	for i := 0; i < num; i++ {
		entries[i] = be.Uint64(b[4+i*8:])
	}
	box.Co64 = &Co64{Entries: entries}
	return nil
}

func encodeCo64(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Co64
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		be.PutUint64(b[4+i*8:], e)
	}
	return 4 + len(s.Entries)*8
}

func encodingLengthCo64(box *Box) int {
	return 4 + len(box.Co64.Entries)*8
}

// --- stts ---

func decodeStts(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]STTSEntry, num)
	for i := 0; i < num; i++ {
		ptr := 4 + i*8
		entries[i] = STTSEntry{
			Count:    be.Uint32(b[ptr:]),
			Duration: be.Uint32(b[ptr+4:]),
		}
	}
	box.Stts = &Stts{Entries: entries}
	return nil
}

func encodeStts(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Stts
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		ptr := 4 + i*8
		be.PutUint32(b[ptr:], e.Count)
		be.PutUint32(b[ptr+4:], e.Duration)
	}
	return 4 + len(s.Entries)*8
}

func encodingLengthStts(box *Box) int {
	return 4 + len(box.Stts.Entries)*8
}

// --- ctts ---

func decodeCtts(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]CTTSEntry, num)
	for i := 0; i < num; i++ {
		ptr := 4 + i*8
		entries[i] = CTTSEntry{
			Count:             be.Uint32(b[ptr:]),
			CompositionOffset: int32(be.Uint32(b[ptr+4:])),
		}
	}
	box.Ctts = &Ctts{Entries: entries}
	return nil
}

func encodeCtts(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Ctts
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		ptr := 4 + i*8
		be.PutUint32(b[ptr:], e.Count)
		be.PutUint32(b[ptr+4:], uint32(e.CompositionOffset))
	}
	return 4 + len(s.Entries)*8
}

func encodingLengthCtts(box *Box) int {
	return 4 + len(box.Ctts.Entries)*8
}

// --- stsc ---

func decodeStsc(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]STSCEntry, num)
	for i := 0; i < num; i++ {
		ptr := 4 + i*12
		entries[i] = STSCEntry{
			FirstChunk:          be.Uint32(b[ptr:]),
			SamplesPerChunk:     be.Uint32(b[ptr+4:]),
			SampleDescriptionId: be.Uint32(b[ptr+8:]),
		}
	}
	box.Stsc = &Stsc{Entries: entries}
	return nil
}

func encodeStsc(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Stsc
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		ptr := 4 + i*12
		be.PutUint32(b[ptr:], e.FirstChunk)
		be.PutUint32(b[ptr+4:], e.SamplesPerChunk)
		be.PutUint32(b[ptr+8:], e.SampleDescriptionId)
	}
	return 4 + len(s.Entries)*12
}

func encodingLengthStsc(box *Box) int {
	return 4 + len(box.Stsc.Entries)*12
}

// --- dref ---

func decodeDref(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]DrefEntry, num)
	ptr := 4
	for i := 0; i < num; i++ {
		size := int(be.Uint32(b[ptr:]))
		var t [4]byte
		copy(t[:], b[ptr+4:ptr+8])
		dataBuf := make([]byte, size-8)
		copy(dataBuf, b[ptr+8:ptr+size])
		entries[i] = DrefEntry{Type: t, Buf: dataBuf}
		ptr += size
	}
	box.Dref = &DrefBox{Entries: entries}
	return nil
}

func encodeDref(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	d := box.Dref
	be.PutUint32(b[0:4], uint32(len(d.Entries)))
	ptr := 4
	for _, e := range d.Entries {
		size := 8 + len(e.Buf)
		be.PutUint32(b[ptr:], uint32(size))
		copy(b[ptr+4:], e.Type[:])
		copy(b[ptr+8:], e.Buf)
		ptr += size
	}
	return ptr
}

func encodingLengthDref(box *Box) int {
	total := 4
	for _, e := range box.Dref.Entries {
		total += 8 + len(e.Buf)
	}
	return total
}

// --- elst ---

func decodeElst(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	entries := make([]ElstEntry, num)
	for i := 0; i < num; i++ {
		ptr := 4 + i*12
		var mr [4]byte
		copy(mr[:], b[ptr+8:ptr+12])
		entries[i] = ElstEntry{
			TrackDuration: be.Uint32(b[ptr:]),
			MediaTime:     int32(be.Uint32(b[ptr+4:])),
			MediaRate:     mr,
		}
	}
	box.Elst = &Elst{Entries: entries}
	return nil
}

func encodeElst(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	s := box.Elst
	be.PutUint32(b[0:4], uint32(len(s.Entries)))
	for i, e := range s.Entries {
		ptr := 4 + i*12
		be.PutUint32(b[ptr:], e.TrackDuration)
		be.PutUint32(b[ptr+4:], uint32(e.MediaTime))
		copy(b[ptr+8:ptr+12], e.MediaRate[:])
	}
	return 4 + len(s.Entries)*12
}

func encodingLengthElst(box *Box) int {
	return 4 + len(box.Elst.Entries)*12
}

// --- hdlr ---

func decodeHdlr(box *Box, buf []byte, start, end int) error {
	b := buf[start:end]
	h := &Hdlr{}
	copy(h.HandlerType[:], b[4:8])
	h.Name = readString(b, 20, end-start)
	box.Hdlr = h
	return nil
}

func encodeHdlr(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	h := box.Hdlr
	nameLen := len(h.Name)
	total := 21 + nameLen
	clearBytes(b, 0, total)
	copy(b[4:8], h.HandlerType[:])
	copy(b[20:], h.Name)
	b[20+nameLen] = 0
	return total
}

func encodingLengthHdlr(box *Box) int {
	return 21 + len(box.Hdlr.Name)
}

// --- mehd ---

func decodeMehd(box *Box, buf []byte, start, _ int) error {
	box.Mehd = &Mehd{FragmentDuration: be.Uint32(buf[start:])}
	return nil
}

func encodeMehd(box *Box, buf []byte, offset int) int {
	be.PutUint32(buf[offset:], box.Mehd.FragmentDuration)
	return 4
}

func encodingLengthMehd(_ *Box) int { return 4 }

// --- trex ---

func decodeTrex(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	box.Trex = &Trex{
		TrackId:                       be.Uint32(b[0:4]),
		DefaultSampleDescriptionIndex: be.Uint32(b[4:8]),
		DefaultSampleDuration:         be.Uint32(b[8:12]),
		DefaultSampleSize:             be.Uint32(b[12:16]),
		DefaultSampleFlags:            be.Uint32(b[16:20]),
	}
	return nil
}

func encodeTrex(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	t := box.Trex
	be.PutUint32(b[0:4], t.TrackId)
	be.PutUint32(b[4:8], t.DefaultSampleDescriptionIndex)
	be.PutUint32(b[8:12], t.DefaultSampleDuration)
	be.PutUint32(b[12:16], t.DefaultSampleSize)
	be.PutUint32(b[16:20], t.DefaultSampleFlags)
	return 20
}

func encodingLengthTrex(_ *Box) int { return 20 }

// --- mfhd ---

func decodeMfhd(box *Box, buf []byte, start, _ int) error {
	box.Mfhd = &Mfhd{SequenceNumber: be.Uint32(buf[start:])}
	return nil
}

func encodeMfhd(box *Box, buf []byte, offset int) int {
	be.PutUint32(buf[offset:], box.Mfhd.SequenceNumber)
	return 4
}

func encodingLengthMfhd(_ *Box) int { return 4 }

// --- tfhd ---

func decodeTfhd(box *Box, buf []byte, start, _ int) error {
	box.Tfhd = &Tfhd{TrackId: be.Uint32(buf[start:])}
	return nil
}

func encodeTfhd(box *Box, buf []byte, offset int) int {
	be.PutUint32(buf[offset:], box.Tfhd.TrackId)
	return 4
}

func encodingLengthTfhd(_ *Box) int { return 4 }

// --- tfdt ---

func decodeTfdt(box *Box, buf []byte, start, _ int) error {
	box.Tfdt = &Tfdt{BaseMediaDecodeTime: be.Uint32(buf[start:])}
	return nil
}

func encodeTfdt(box *Box, buf []byte, offset int) int {
	be.PutUint32(buf[offset:], box.Tfdt.BaseMediaDecodeTime)
	return 4
}

func encodingLengthTfdt(_ *Box) int { return 4 }

// --- trun ---

func decodeTrun(box *Box, buf []byte, start, _ int) error {
	b := buf[start:]
	num := int(be.Uint32(b[0:4]))
	t := &Trun{
		DataOffset: int32(be.Uint32(b[4:8])),
		Entries:    make([]TrunEntry, num),
	}
	ptr := 8
	for i := 0; i < num; i++ {
		e := TrunEntry{
			SampleDuration:              be.Uint32(b[ptr:]),
			SampleSize:                  be.Uint32(b[ptr+4:]),
			SampleFlags:                 be.Uint32(b[ptr+8:]),
			SampleCompositionTimeOffset: int32(be.Uint32(b[ptr+12:])),
		}
		t.Entries[i] = e
		ptr += 16
	}
	box.Trun = t
	return nil
}

func encodeTrun(box *Box, buf []byte, offset int) int {
	b := buf[offset:]
	t := box.Trun
	be.PutUint32(b[0:4], uint32(len(t.Entries)))
	be.PutUint32(b[4:8], uint32(t.DataOffset))
	ptr := 8
	for _, e := range t.Entries {
		be.PutUint32(b[ptr:], e.SampleDuration)
		be.PutUint32(b[ptr+4:], e.SampleSize)
		be.PutUint32(b[ptr+8:], e.SampleFlags)
		be.PutUint32(b[ptr+12:], uint32(e.SampleCompositionTimeOffset))
		ptr += 16
	}
	return ptr
}

func encodingLengthTrun(box *Box) int {
	return 8 + len(box.Trun.Entries)*16
}

// --- mdat ---

func decodeMdat(box *Box, buf []byte, start, end int) error {
	b := make([]byte, end-start)
	copy(b, buf[start:end])
	box.Mdat = &Mdat{Buffer: b}
	return nil
}

func encodeMdat(box *Box, buf []byte, offset int) int {
	m := box.Mdat
	if m.Buffer != nil {
		copy(buf[offset:], m.Buffer)
		return len(m.Buffer)
	}
	return m.ContentLength
}

func encodingLengthMdat(box *Box) int {
	m := box.Mdat
	if m.Buffer != nil {
		return len(m.Buffer)
	}
	return m.ContentLength
}
