package mp4

// descriptor implements MPEG-4 descriptor parsing for esds boxes.

var tagToName = map[byte]string{
	0x03: "ESDescriptor",
	0x04: "DecoderConfigDescriptor",
	0x05: "DecoderSpecificInfo",
	0x06: "SLConfigDescriptor",
}

type descriptor struct {
	tag      byte
	tagName  string
	length   int
	oti      byte
	buffer   []byte
	children map[string]*descriptor
}

func decodeDescriptor(buf []byte, start, end int) *descriptor {
	if start >= end {
		return nil
	}
	tag := buf[start]
	ptr := start + 1
	length := 0
	for ptr < end {
		lenByte := buf[ptr]
		ptr++
		length = (length << 7) | int(lenByte&0x7f)
		if lenByte&0x80 == 0 {
			break
		}
	}

	tagName := tagToName[tag]
	d := &descriptor{
		tag:      tag,
		tagName:  tagName,
		length:   (ptr - start) + length,
		children: make(map[string]*descriptor),
	}

	switch tagName {
	case "ESDescriptor":
		decodeESDescriptor(d, buf, ptr, end)
	case "DecoderConfigDescriptor":
		decodeDecoderConfigDescriptor(d, buf, ptr, end)
	case "DecoderSpecificInfo":
		dEnd := ptr + length
		if dEnd > end {
			dEnd = end
		}
		d.buffer = buf[ptr:dEnd]
	default:
		dEnd := min(ptr+length, end)
		d.buffer = buf[ptr:dEnd]
	}

	return d
}

func decodeDescriptorArray(buf []byte, start, end int) map[string]*descriptor {
	m := make(map[string]*descriptor)
	ptr := start
	for ptr+2 <= end {
		desc := decodeDescriptor(buf, ptr, end)
		if desc == nil {
			break
		}
		ptr += desc.length
		name := desc.tagName
		if name == "" {
			continue
		}
		m[name] = desc
	}
	return m
}

func decodeESDescriptor(d *descriptor, buf []byte, start, end int) {
	if start+3 > end {
		return
	}
	flags := buf[start+2]
	ptr := start + 3
	if flags&0x80 != 0 {
		ptr += 2
	}
	if flags&0x40 != 0 {
		if ptr >= end {
			return
		}
		l := int(buf[ptr])
		ptr += l + 1
	}
	if flags&0x20 != 0 {
		ptr += 2
	}
	d.children = decodeDescriptorArray(buf, ptr, end)
}

func decodeDecoderConfigDescriptor(d *descriptor, buf []byte, start, end int) {
	if start >= end {
		return
	}
	d.oti = buf[start]
	d.children = decodeDescriptorArray(buf, start+13, end)
}
