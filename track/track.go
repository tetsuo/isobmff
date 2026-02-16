package track

import (
	"errors"
	"fmt"

	"github.com/tetsuo/mp4"
)

// TrackKind distinguishes video and audio tracks.
type TrackKind int

const (
	TrackVideo TrackKind = iota
	TrackAudio
)

// trackRaw holds internal parsing state and raw box data.
type trackRaw struct {
	// Raw sub-slices of moov buffer.
	stsd []byte
	tkhd []byte // tkhd data (after version+flags header)
	mdhd []byte // mdhd data (after version+flags header)
	hdlr []byte // entire hdlr raw box
	dinf []byte // entire dinf raw box

	tkhdVersion uint8
	tkhdFlags   uint32
	mdhdVersion uint8
	hasVmhd     bool
	hasDinf     bool

	// Raw sample table data.
	stszData    []byte
	sttsData    []byte
	stscData    []byte
	cttsData    []byte
	cttsVersion uint8
	stssData    []byte
	stcoData    []byte
	co64Data    []byte
	hasCo64     bool
	sampleCount uint32

	// Codec string builder buffer.
	codecBuf [24]byte
	codecLen uint8
}

// Track holds metadata for one track parsed from a moov box.
type Track struct {
	ID        uint32
	Kind      TrackKind
	TimeScale uint32
	Duration  uint64

	Width        uint16
	Height       uint16
	ChannelCount uint16
	SampleRate   uint32

	Samples       []Sample
	SampleDescIdx uint32

	codec string // set once after parsing completes
	raw   trackRaw
}

// Codec returns the MIME codec string (e.g. "avc1.64001e", "mp4a.40.2").
func (t *Track) Codec() string { return t.codec }

// StsdRaw returns the raw stsd box data (entire box including header).
func (t *Track) StsdRaw() []byte { return t.raw.stsd }

// TkhdRaw returns the tkhd box data (after version+flags header).
func (t *Track) TkhdRaw() []byte { return t.raw.tkhd }

// MdhdRaw returns the mdhd box data (after version+flags header).
func (t *Track) MdhdRaw() []byte { return t.raw.mdhd }

// HdlrRaw returns the entire hdlr raw box.
func (t *Track) HdlrRaw() []byte { return t.raw.hdlr }

// DinfRaw returns the entire dinf raw box.
func (t *Track) DinfRaw() []byte { return t.raw.dinf }

// TkhdVersion returns the version field of the tkhd box.
func (t *Track) TkhdVersion() uint8 { return t.raw.tkhdVersion }

// TkhdFlags returns the flags field of the tkhd box.
func (t *Track) TkhdFlags() uint32 { return t.raw.tkhdFlags }

// MdhdVersion returns the version field of the mdhd box.
func (t *Track) MdhdVersion() uint8 { return t.raw.mdhdVersion }

// HasVmhd returns true if the track has a vmhd box (video media header).
func (t *Track) HasVmhd() bool { return t.raw.hasVmhd }

// HasDinf returns true if the track has a dinf box (data information).
func (t *Track) HasDinf() bool { return t.raw.hasDinf }

// FindTrack returns the track with the given ID, or nil.
func FindTrack(tracks []*Track, id uint32) *Track {
	for _, t := range tracks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (t *Track) setCodec(s string) {
	n := copy(t.raw.codecBuf[:], s)
	t.raw.codecLen = uint8(n)
}

func (t *Track) appendCodec(s string) {
	n := copy(t.raw.codecBuf[t.raw.codecLen:], s)
	t.raw.codecLen += uint8(n)
}

// appendAvcCProfile appends "XXYYZZ" hex profile to the codec buffer.
func (t *Track) appendAvcCProfile(profile, compat, level byte) {
	i := t.raw.codecLen
	t.raw.codecBuf[i+0] = hexChars[profile>>4]
	t.raw.codecBuf[i+1] = hexChars[profile&0x0f]
	t.raw.codecBuf[i+2] = hexChars[compat>>4]
	t.raw.codecBuf[i+3] = hexChars[compat&0x0f]
	t.raw.codecBuf[i+4] = hexChars[level>>4]
	t.raw.codecBuf[i+5] = hexChars[level&0x0f]
	t.raw.codecLen += 6
}

// appendEsdsCodec appends ".OTI.audioConfig" to the codec buffer from esds data.
func (t *Track) appendEsdsCodec(data []byte) {
	oti, audioConfig := parseEsds(data)
	if oti == 0 {
		return
	}
	t.appendCodec(".")
	if oti >= 16 {
		t.raw.codecBuf[t.raw.codecLen] = hexChars[oti>>4]
		t.raw.codecLen++
	}
	t.raw.codecBuf[t.raw.codecLen] = hexChars[oti&0x0f]
	t.raw.codecLen++
	if audioConfig > 0 {
		t.appendCodec(".")
		if audioConfig >= 10 {
			t.raw.codecBuf[t.raw.codecLen] = '0' + audioConfig/10
			t.raw.codecLen++
		}
		t.raw.codecBuf[t.raw.codecLen] = '0' + audioConfig%10
		t.raw.codecLen++
	}
}

// parseEsds extracts OTI and audio object type from esds box data.
func parseEsds(data []byte) (oti, audioConfig byte) {
	if len(data) < 2 {
		return 0, 0
	}
	ptr, end := 0, len(data)
	if data[ptr] != 0x03 {
		return 0, 0
	}
	ptr++
	ptr = skipDescLen(data, ptr, end)
	if ptr < 0 || ptr+3 > end {
		return 0, 0
	}
	flags := data[ptr+2]
	ptr += 3
	if flags&0x80 != 0 {
		ptr += 2
	}
	if flags&0x40 != 0 {
		if ptr >= end {
			return 0, 0
		}
		ptr += 1 + int(data[ptr])
	}
	if flags&0x20 != 0 {
		ptr += 2
	}
	if ptr >= end || data[ptr] != 0x04 {
		return 0, 0
	}
	ptr++
	ptr = skipDescLen(data, ptr, end)
	if ptr < 0 || ptr+13 > end {
		return 0, 0
	}
	oti = data[ptr]
	if oti == 0 {
		return 0, 0
	}
	ptr += 13
	if ptr >= end || data[ptr] != 0x05 {
		return oti, 0
	}
	ptr++
	ptr = skipDescLen(data, ptr, end)
	if ptr < 0 || ptr >= end {
		return oti, 0
	}
	audioConfig = (data[ptr] & 0xf8) >> 3
	return oti, audioConfig
}

func skipDescLen(data []byte, ptr, end int) int {
	for ptr < end {
		b := data[ptr]
		ptr++
		if b&0x80 == 0 {
			return ptr
		}
	}
	return -1
}

// Sample represents a single media sample.
type Sample struct {
	TrackID            uint32
	Offset             int64
	Size               uint32
	Duration           uint32
	DTS                int64
	PresentationOffset int32
	IsSync             bool
}

// PTS returns the presentation timestamp.
func (s Sample) PTS() int64 {
	return s.DTS + int64(s.PresentationOffset)
}

// TrackSampleStats holds aggregated stats for samples belonging to one track.
type TrackSampleStats struct {
	TrackID     uint32
	TimeScale   uint32
	Duration    uint64
	EarliestPTS int64
	SampleCount int
}

// CollectTrackSampleStats aggregates sample count, duration, and earliest PTS
// per track. The returned slice contains only tracks that have at least one sample.
func CollectTrackSampleStats(dst []TrackSampleStats, tracks []*Track, samples []Sample) []TrackSampleStats {
	if cap(dst) < len(tracks) {
		dst = make([]TrackSampleStats, len(tracks))
	} else {
		dst = dst[:len(tracks)]
	}

	for i, t := range tracks {
		dst[i] = TrackSampleStats{
			TrackID:     t.ID,
			TimeScale:   t.TimeScale,
			EarliestPTS: -1,
		}
	}

	for i := range samples {
		s := &samples[i]
		for j := range dst {
			if dst[j].TrackID != s.TrackID {
				continue
			}
			st := &dst[j]
			st.SampleCount++
			st.Duration += uint64(s.Duration)
			pts := s.PTS()
			if st.EarliestPTS < 0 || pts < st.EarliestPTS {
				st.EarliestPTS = pts
			}
			break
		}
	}

	out := dst[:0]
	for i := range dst {
		if dst[i].SampleCount > 0 {
			out = append(out, dst[i])
		}
	}
	return out
}

var (
	htVide = [4]byte{'v', 'i', 'd', 'e'}
	htSoun = [4]byte{'s', 'o', 'u', 'n'}
)

var (
	ErrMoovNotFound = errors.New("moov box not found in buffer")
	ErrInvalidTrack = errors.New("invalid track data")
	ErrCorruptData  = errors.New("corrupt data")
)

// ParseTracks parses a moov box buffer and returns the tracks found with
// their samples fully populated. The moov buffer must include the box header
// (the full top-level moov box). The movie duration (from mvhd) is also returned.
//
// Returns an error if the moov box is not found, if no playable tracks are
// found, or if sample tables cannot be parsed for any track.
func ParseTracks(moovBuf []byte) ([]*Track, uint64, error) {
	mr := mp4.NewReader(moovBuf)
	if !mr.Next() || mr.Type() != mp4.TypeMoov {
		return nil, 0, ErrMoovNotFound
	}

	var tracks []*Track
	var duration uint64

	mr.Enter()
	for mr.Next() {
		switch mr.Type() {
		case mp4.TypeMvhd:
			_, dur, _ := mr.ReadMvhd()
			duration = dur
		case mp4.TypeTrak:
			track := parseTrak(&mr)
			if track != nil {
				tracks = append(tracks, track)
			}
		}
	}
	mr.Exit()

	// Parse samples for all tracks; filter out those that fail
	var valid []*Track
	for _, t := range tracks {
		if err := t.parseSamples(); err != nil {
			continue
		}
		valid = append(valid, t)
	}

	return valid, duration, nil
}

func parseTrak(mr *mp4.Reader) *Track {
	track := &Track{}

	mr.Enter()
	defer mr.Exit()

	for mr.Next() {
		switch mr.Type() {
		case mp4.TypeTkhd:
			track.raw.tkhdVersion = mr.Version()
			track.raw.tkhdFlags = mr.Flags()
			track.raw.tkhd = mr.Data()
			trackId, _, w, h := mr.ReadTkhd()
			track.ID = trackId
			track.Width = uint16(w >> 16)
			track.Height = uint16(h >> 16)
		case mp4.TypeMdia:
			parseMdia(mr, track)
		}
	}

	if track.ID == 0 || track.raw.codecLen == 0 {
		return nil
	}

	// Finalize codec string
	track.codec = string(track.raw.codecBuf[:track.raw.codecLen])

	return track
}

func parseMdia(mr *mp4.Reader, track *Track) {
	mr.Enter()
	defer mr.Exit()

	var handlerType [4]byte

	for mr.Next() {
		switch mr.Type() {
		case mp4.TypeMdhd:
			track.raw.mdhdVersion = mr.Version()
			track.raw.mdhd = mr.Data()
			ts, dur, _ := mr.ReadMdhd()
			track.TimeScale = ts
			track.Duration = dur
		case mp4.TypeHdlr:
			track.raw.hdlr = mr.RawBox()
			handlerType = mr.ReadHdlr()
		case mp4.TypeMinf:
			parseMinf(mr, track, handlerType)
		}
	}
}

func parseMinf(mr *mp4.Reader, track *Track, handlerType [4]byte) {
	mr.Enter()
	defer mr.Exit()

	for mr.Next() {
		switch mr.Type() {
		case mp4.TypeVmhd:
			track.raw.hasVmhd = true
		case mp4.TypeSmhd:
			track.raw.hasVmhd = false
		case mp4.TypeDinf:
			track.raw.hasDinf = true
			track.raw.dinf = mr.RawBox()
		case mp4.TypeStbl:
			parseStbl(mr, track, handlerType)
		}
	}
}

func parseStbl(mr *mp4.Reader, track *Track, handlerType [4]byte) {
	mr.Enter()
	defer mr.Exit()

	for mr.Next() {
		switch mr.Type() {
		case mp4.TypeStsd:
			track.raw.stsd = mr.RawBox()
			parseStsd(mr, track, handlerType)
		case mp4.TypeStsz:
			track.raw.stszData = mr.Data()
		case mp4.TypeStts:
			track.raw.sttsData = mr.Data()
		case mp4.TypeStsc:
			track.raw.stscData = mr.Data()
		case mp4.TypeCtts:
			track.raw.cttsData = mr.Data()
			track.raw.cttsVersion = mr.Version()
		case mp4.TypeStss:
			track.raw.stssData = mr.Data()
		case mp4.TypeStco:
			track.raw.stcoData = mr.Data()
		case mp4.TypeCo64:
			track.raw.co64Data = mr.Data()
			track.raw.hasCo64 = true
		}
	}

	if track.raw.stszData != nil {
		stszIt := mp4.NewStszIter(track.raw.stszData)
		track.raw.sampleCount = stszIt.Count()
	}

	// Extract SampleDescIdx from first stsc entry (needed for init segment writing)
	if track.raw.stscData != nil {
		stscIt := mp4.NewStscIter(track.raw.stscData)
		if entry, ok := stscIt.Next(); ok {
			track.SampleDescIdx = entry.SampleDescriptionId
		}
	}
}

func parseStsd(mr *mp4.Reader, track *Track, handlerType [4]byte) {
	data := mr.Data()
	if len(data) < 4 {
		return
	}

	mr.Enter()
	mr.Skip(4)

	if !mr.Next() {
		mr.Exit()
		return
	}

	entryType := mr.Type()
	entryData := mr.Data()

	if handlerType == htVide && entryType == mp4.TypeAvc1 {
		track.Kind = TrackVideo
		track.setCodec("avc1")
		if len(entryData) >= 78 {
			v := mp4.ReadVisualSampleEntry(entryData)
			track.Width = v.Width
			track.Height = v.Height

			mr.Enter()
			mr.Skip(v.ChildOffset)
			for mr.Next() {
				if mr.Type() == mp4.TypeAvcC {
					d := mr.Data()
					if len(d) >= 4 {
						track.appendCodec(".")
						track.appendAvcCProfile(d[1], d[2], d[3])
					}
					break
				}
			}
			mr.Exit()
		}
	} else if handlerType == htSoun && entryType == mp4.TypeMp4a {
		track.Kind = TrackAudio
		track.setCodec("mp4a")
		if len(entryData) >= 28 {
			a := mp4.ReadAudioSampleEntry(entryData)
			track.ChannelCount = a.ChannelCount
			track.SampleRate = a.SampleRate >> 16

			mr.Enter()
			mr.Skip(a.ChildOffset)
			for mr.Next() {
				if mr.Type() == mp4.TypeEsds {
					track.appendEsdsCodec(mr.Data())
					break
				}
			}
			mr.Exit()
		}
	}

	mr.Exit()
}

// parseSamples parses sample table data and populates track.Samples.
// Returns an error if required sample table data is missing or corrupt.
func (t *Track) parseSamples() error {
	if t.Samples != nil {
		return nil // already parsed
	}
	if t.raw.stszData == nil || t.raw.sttsData == nil || t.raw.stscData == nil {
		return fmt.Errorf("track %d: %w: missing required sample table data (stsz/stts/stsc)", t.ID, ErrInvalidTrack)
	}
	if t.raw.stcoData == nil && t.raw.co64Data == nil {
		return fmt.Errorf("track %d: %w: missing chunk offset data (stco/co64)", t.ID, ErrInvalidTrack)
	}

	stszIt := mp4.NewStszIter(t.raw.stszData)
	numSamples := int(stszIt.Count())
	if numSamples == 0 {
		t.Samples = []Sample{}
		return nil
	}

	samples := make([]Sample, numSamples)

	stscIt := mp4.NewStscIter(t.raw.stscData)
	sttsIt := mp4.NewSttsIter(t.raw.sttsData)

	var cttsIt mp4.CttsIter
	hasCtts := t.raw.cttsData != nil
	if hasCtts {
		cttsIt = mp4.NewCttsIter(t.raw.cttsData, t.raw.cttsVersion)
	}

	hasSync := t.raw.stssData != nil
	var syncIt mp4.Uint32Iter
	if hasSync {
		syncIt = mp4.NewUint32Iter(t.raw.stssData)
	}

	curStsc, ok := stscIt.Next()
	if !ok {
		return fmt.Errorf("track %d: %w: empty stsc table", t.ID, ErrInvalidTrack)
	}
	var nextStsc mp4.StscEntry
	haveNextStsc := false
	if e, ok := stscIt.Next(); ok {
		nextStsc = e
		haveNextStsc = true
	}

	curStts, ok := sttsIt.Next()
	if !ok {
		return fmt.Errorf("track %d: %w: empty stts table", t.ID, ErrInvalidTrack)
	}
	sttsRemaining := int(curStts.Count)

	var curCtts mp4.CttsEntry
	cttsRemaining := 0
	if hasCtts {
		if e, ok := cttsIt.Next(); ok {
			curCtts = e
			cttsRemaining = int(e.Count)
		}
	}

	var nextSync uint32
	haveSync := false
	if hasSync {
		if v, ok := syncIt.Next(); ok {
			nextSync = v
			haveSync = true
		}
	}

	var chunkOffset int64
	var chunkIdx uint32

	var stcoIt mp4.Uint32Iter
	var co64It mp4.Co64Iter
	if t.raw.hasCo64 {
		co64It = mp4.NewCo64Iter(t.raw.co64Data)
		if v, ok := co64It.Next(); ok {
			chunkOffset = int64(v)
		}
	} else {
		stcoIt = mp4.NewUint32Iter(t.raw.stcoData)
		if v, ok := stcoIt.Next(); ok {
			chunkOffset = int64(v)
		}
	}
	chunkIdx = 1

	sampleInChunk := uint32(0)
	var offsetInChunk int64
	var dts int64

	for i := range numSamples {
		size, ok := stszIt.Next()
		if !ok {
			return fmt.Errorf("track %d: %w: stsz iterator exhausted at sample %d/%d", t.ID, ErrCorruptData, i, numSamples)
		}

		var presOff int32
		if hasCtts && cttsRemaining > 0 {
			presOff = curCtts.Offset
		}

		isSync := true
		if hasSync {
			isSync = haveSync && nextSync == uint32(i+1)
		}

		samples[i] = Sample{
			TrackID:            t.ID,
			Offset:             offsetInChunk + chunkOffset,
			Size:               size,
			Duration:           curStts.Duration,
			DTS:                dts,
			PresentationOffset: presOff,
			IsSync:             isSync,
		}

		if i+1 >= numSamples {
			break
		}

		sampleInChunk++
		offsetInChunk += int64(size)
		if sampleInChunk >= curStsc.SamplesPerChunk {
			sampleInChunk = 0
			offsetInChunk = 0
			chunkIdx++
			if t.raw.hasCo64 {
				if v, ok := co64It.Next(); ok {
					chunkOffset = int64(v)
				}
			} else {
				if v, ok := stcoIt.Next(); ok {
					chunkOffset = int64(v)
				}
			}
			if haveNextStsc && chunkIdx >= nextStsc.FirstChunk {
				curStsc = nextStsc
				if e, ok := stscIt.Next(); ok {
					nextStsc = e
				} else {
					haveNextStsc = false
				}
			}
		}

		dts += int64(curStts.Duration)
		sttsRemaining--
		if sttsRemaining <= 0 {
			if e, ok := sttsIt.Next(); ok {
				curStts = e
				sttsRemaining = int(e.Count)
			}
		}

		if hasCtts {
			cttsRemaining--
			if cttsRemaining <= 0 {
				if e, ok := cttsIt.Next(); ok {
					curCtts = e
					cttsRemaining = int(e.Count)
				}
			}
		}

		if isSync && hasSync {
			if v, ok := syncIt.Next(); ok {
				nextSync = v
			} else {
				haveSync = false
			}
		}
	}

	t.Samples = samples
	t.SampleDescIdx = curStsc.SampleDescriptionId
	return nil
}

const hexChars = "0123456789abcdef"
