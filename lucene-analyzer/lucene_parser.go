package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------- low-level readers for Lucene DataInput style ----------

const (
	CODEC_MAGIC       = 0x3fd76c17
	ID_LENGTH         = 16 // Lucene StringHelper.ID_LENGTH == 16
	SEGMENTS_PREFIX   = "segments"
	SEGMENTS_GEN_FILE = "segments.gen"
)

func readExactly(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func readBEInt32(r io.Reader) (int32, error) {
	b, err := readExactly(r, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(b)), nil
}

func readBELong(r io.Reader) (int64, error) {
	b, err := readExactly(r, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

func readByte(r io.Reader) (byte, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func readVInt(r io.Reader) (int, error) {
	var result int
	var shift uint
	for i := 0; i < 5; i++ {
		b, err := readByte(r)
		if err != nil {
			return 0, err
		}
		result |= int(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
	return 0, errors.New("invalid vInt (too long)")
}

func readVLong(r io.Reader) (int64, error) {
	var result int64
	var shift uint
	for i := 0; i < 10; i++ {
		b, err := readByte(r)
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
	return 0, errors.New("invalid vLong (too long)")
}

func readString(r io.Reader) (string, error) {
	n, err := readVInt(r)
	if err != nil {
		return "", err
	}
	if n < 0 {
		return "", errors.New("negative string length")
	}
	if n == 0 {
		return "", nil
	}
	b, err := readExactly(r, n)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readSetOfStrings(r io.Reader) ([]string, error) {
	cnt, err := readVInt(r)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, cnt)
	for i := 0; i < cnt; i++ {
		s, err := readString(r)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func readMapOfStrings(r io.Reader) (map[string]string, error) {
	cnt, err := readVInt(r)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, cnt)
	for i := 0; i < cnt; i++ {
		k, err := readString(r)
		if err != nil {
			return nil, err
		}
		v, err := readString(r)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// ---------- helpers to find latest segments_N file ----------

func generationFromSegmentsFileName(name string) (int64, error) {
	if !strings.HasPrefix(name, SEGMENTS_PREFIX+"_") {
		return -1, fmt.Errorf("bad segments name: %s", name)
	}
	suffix := name[len(SEGMENTS_PREFIX)+1:]
	val, err := strconv.ParseInt(suffix, 36, 64)
	if err != nil {
		return -1, err
	}
	return val, nil
}

func findLatestSegmentsFile(dir string) (string, error) {
	fis, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var bestName string
	var bestGen int64 = -1
	for _, fi := range fis {
		name := fi.Name()
		if !strings.HasPrefix(name, SEGMENTS_PREFIX) {
			continue
		}
		if name == SEGMENTS_GEN_FILE {
			continue
		}
		if name == SEGMENTS_PREFIX {
			if bestGen < 0 {
				bestGen = 0
				bestName = name
			}
			continue
		}
		gen, err := generationFromSegmentsFileName(name)
		if err != nil {
			continue
		}
		if gen > bestGen {
			bestGen = gen
			bestName = name
		}
	}
	if bestName == "" {
		return "", errors.New("no segments_N file found")
	}
	return bestName, nil
}

// ---------- parse .si (SegmentInfo) per Lucene90SegmentInfoFormat ----------

type Version struct {
	Major, Minor, Bugfix int32
}

type SegmentInfo struct {
	Name           string
	ID             []byte
	Version        Version
	MinVersion     *Version
	DocCount       int32
	IsCompoundFile bool
	Diagnostics    map[string]string
	Files          []string
	Attributes     map[string]string
}

func readMap(r io.Reader) (map[string]string, error) {
	sz, err := readVInt(r)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, sz)
	for i := int(0); i < sz; i++ {
		k, _ := readString(r)
		v, _ := readString(r)
		m[k] = v
	}
	return m, nil
}

// parseSegmentSI is now in lucene_parser.go

// ---------- parse segments_N (SegmentInfos) ----------

type SegInfoSummary struct {
	SegName       string            `json:"name"`
	SegID         string            `json:"seg_id"` // hex
	SegCodec      string            `json:"codec"`
	MaxDoc        int32             `json:"max_doc"`
	Compound      bool              `json:"compound"`
	Files         []string          `json:"files,omitempty"`
	DelGen        int64             `json:"del_gen"`
	DelCount      int32             `json:"del_count"`
	FieldInfosGen int64             `json:"field_infos_gen"`
	DVGen         int64             `json:"dv_gen"`
	SoftDelCount  int32             `json:"soft_del_count"`
	SciID         string            `json:"sci_id,omitempty"`
	Extra         map[string]string `json:"diagnostics,omitempty"`
}

// parseSegmentsFile is now in lucene_parser.go

// ---------- report building and printing ----------

type Report struct {
	IndexPath            string            `json:"index_path"`
	SegmentsFile         string            `json:"segments_file"`
	TotalSegments        int               `json:"total_segments"`
	TotalDocs            int64             `json:"total_docs"`
	TotalDeletedDocs     int64             `json:"total_deleted_docs"`
	TotalSoftDeletedDocs int64             `json:"total_soft_deleted_docs"`
	UserData             map[string]string `json:"user_data,omitempty"`
	Segments             []SegInfoSummary  `json:"segments"`
	Notes                string            `json:"notes,omitempty"`
}

func buildReport(indexDir string) (*Report, error) {
	segFile, err := findLatestSegmentsFile(indexDir)
	if err != nil {
		return nil, err
	}
	summaries, userData, err := parseSegmentsFile(indexDir, segFile)
	if err != nil {
		return nil, err
	}
	var totalDocs int64
	var totalDeleted int64
	var totalSoftDeleted int64
	for _, s := range summaries {
		totalDocs += int64(s.MaxDoc)
		totalDeleted += int64(s.DelCount)
		totalSoftDeleted += int64(s.SoftDelCount) // 累加软删除数量
	}
	rep := &Report{
		IndexPath:            indexDir,
		SegmentsFile:         segFile,
		TotalSegments:        len(summaries),
		TotalDocs:            totalDocs,
		TotalDeletedDocs:     totalDeleted,
		TotalSoftDeletedDocs: totalSoftDeleted,
		UserData:             userData,
		Segments:             summaries,
		Notes:                "Parsed per Lucene90SegmentInfoFormat: segVersion (string), maxDoc (int32), isCompound (byte), diagnostics, files, attributes.",
	}
	return rep, nil
}

// parseSegmentSI 读取并解析 .si 文件 (LittleEndian 用于文档数)
func parseSegmentSI(indexDir, segName string) (int32, bool, map[string]string, error) {
	path := filepath.Join(indexDir, segName+".si")
	f, err := os.Open(path)
	if err != nil {
		return 0, false, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	// 跳过 Header: Magic(4), Codec(String), Ver(4), ID(16), Suffix(String)
	readBEInt32(r)
	readString(r)
	readBEInt32(r)
	readExactly(r, 16)
	readString(r)

	// 读取版本和可选版本 (LittleEndian)
	var v Version
	binary.Read(r, binary.LittleEndian, &v)
	var hasMin byte
	binary.Read(r, binary.LittleEndian, &hasMin)
	if hasMin == 1 {
		binary.Read(r, binary.LittleEndian, &v)
	}

	var docCount int32
	binary.Read(r, binary.LittleEndian, &docCount)
	isCompound, _ := readByte(r)
	diag, _ := readMap(r)

	return docCount, isCompound == 1, diag, nil
}

// parseSegmentsFile 解析 segments_N 文件并提取软删除数量
func parseSegmentsFile(indexDir, segFile string) ([]SegInfoSummary, map[string]string, error) {
	f, err := os.Open(filepath.Join(indexDir, segFile))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	// 1. 解析 Header
	magic, _ := readBEInt32(r)
	if magic != CODEC_MAGIC {
		return nil, nil, errors.New("bad segments magic")
	}
	readString(r) // "segments"
	formatVer, _ := readBEInt32(r)
	readExactly(r, 16) // ID
	sufLen, _ := readByte(r)
	readExactly(r, int(sufLen)) // Suffix

	// 2. 解析 Lucene 版本信息
	readByte(r)
	readByte(r)
	readByte(r) // Version Triple
	readByte(r) // Index Created Version

	// 3. 统计信息
	readBELong(r) // SegInfo Version
	readVLong(r)  // Counter
	numSegs, _ := readBEInt32(r)

	if numSegs > 0 {
		readByte(r)
		readByte(r)
		readByte(r) // Min Segment Version
	}

	var summaries []SegInfoSummary
	for i := 0; i < int(numSegs); i++ {
		name, _ := readString(r)
		segIDBytes, _ := readExactly(r, ID_LENGTH) // ID
		codec, _ := readString(r)

		// 获取段详细信息
		maxDoc, isCompound, diag, _ := parseSegmentSI(indexDir, name)

		// 读取删除和软删除计数
		delGen, _ := readBELong(r)
		delCount, _ := readBEInt32(r)
		fieldInfosGen, _ := readBELong(r)
		dvGen, _ := readBELong(r)
		softDelCount, _ := readBEInt32(r)

		// 处理 SCI ID (format > 9)
		var sciIdBytes []byte
		if formatVer > 9 {
			marker, _ := readByte(r)
			if marker == 1 {
				sciIdBytes, _ = readExactly(r, ID_LENGTH)
			}
		}

		// 跳过文件列表和 DV 更新信息
		readSetOfStrings(r)
		numDV, _ := readBEInt32(r)
		for j := 0; j < int(numDV); j++ {
			readBEInt32(r)
			readSetOfStrings(r)
		}

		summary := SegInfoSummary{
			SegName:       name,
			SegID:         hex.EncodeToString(segIDBytes),
			SegCodec:      codec,
			MaxDoc:        maxDoc,
			Compound:      isCompound,
			DelGen:        delGen,
			DelCount:      delCount,
			FieldInfosGen: fieldInfosGen,
			DVGen:         dvGen,
			SoftDelCount:  softDelCount,
			Extra:         diag,
		}
		if len(sciIdBytes) > 0 {
			summary.SciID = hex.EncodeToString(sciIdBytes)
		}
		summaries = append(summaries, summary)
	}

	userData, _ := readMapOfStrings(r)
	return summaries, userData, nil
}
