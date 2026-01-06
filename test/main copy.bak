package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	CODEC_MAGIC     = 0x3fd76c17
	ID_LENGTH       = 16
	SEGMENTS_PREFIX = "segments"
)

// ---------- 基础读取工具 ----------

func readExactly(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func readByte(r io.Reader) (byte, error) {
	b, err := readExactly(r, 1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func readBEInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readBELong(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readVInt(r io.Reader) (int32, error) {
	var result int32
	var shift uint
	for i := 0; i < 5; i++ {
		b, err := readByte(r)
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
	return 0, errors.New("vInt too long")
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
	return 0, errors.New("vLong too long")
}

func readString(r io.Reader) (string, error) {
	n, err := readVInt(r)
	if err != nil || n <= 0 {
		return "", err
	}
	b, err := readExactly(r, int(n))
	return string(b), err
}

func readSet(r io.Reader) ([]string, error) {
	cnt, err := readVInt(r)
	if err != nil {
		return nil, err
	}
	res := make([]string, cnt)
	for i := 0; i < int(cnt); i++ {
		res[i], _ = readString(r)
	}
	return res, nil
}

func readMap(r io.Reader) (map[string]string, error) {
	cnt, err := readVInt(r)
	if err != nil {
		return nil, err
	}
	res := make(map[string]string, cnt)
	for i := 0; i < int(cnt); i++ {
		k, _ := readString(r)
		v, _ := readString(r)
		res[k] = v
	}
	return res, nil
}

// ---------- 核心解析结构 ----------

type Version struct {
	Major, Minor, Bugfix int32 `json:"major"`
}

type SegSummary struct {
	Name         string            `json:"name"`
	Codec        string            `json:"codec"`
	MaxDoc       int32             `json:"max_doc"`
	DelCount     int32             `json:"del_count"`
	SoftDelCount int32             `json:"soft_del_count"` // 新增软删除字段
	IsCompound   bool              `json:"is_compound"`
	Diagnostics  map[string]string `json:"diagnostics,omitempty"`
}

type Report struct {
	IndexPath            string            `json:"index_path"`
	SegmentsFile         string            `json:"segments_file"`
	TotalSegments        int               `json:"total_segments"`
	TotalDocs            int64             `json:"total_docs"`
	TotalDeletedDocs     int64             `json:"total_deleted_docs"`
	TotalSoftDeletedDocs int64             `json:"total_soft_deleted_docs"`
	UserData             map[string]string `json:"user_data,omitempty"`
	Segments             []SegSummary      `json:"segments"`
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
func parseSegmentsFile(indexDir, fileName string) ([]SegSummary, map[string]string, error) {
	f, err := os.Open(filepath.Join(indexDir, fileName))
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

	var summaries []SegSummary
	for i := 0; i < int(numSegs); i++ {
		name, _ := readString(r)
		readExactly(r, 16) // ID
		codec, _ := readString(r)

		// 获取段详细信息
		maxDoc, isCompound, diag, _ := parseSegmentSI(indexDir, name)

		// 读取删除和软删除计数
		readBELong(r) // delGen
		delCount, _ := readBEInt32(r)
		readBELong(r) // fieldInfosGen
		readBELong(r) // dvGen
		softDelCount, _ := readBEInt32(r)

		// 处理 SCI ID (format > 9)
		if formatVer > 9 {
			marker, _ := readByte(r)
			if marker == 1 {
				readExactly(r, 16)
			}
		}

		// 跳过文件列表和 DV 更新信息
		readSet(r)
		numDV, _ := readBEInt32(r)
		for j := 0; j < int(numDV); j++ {
			readBEInt32(r)
			readSet(r)
		}

		summaries = append(summaries, SegSummary{
			Name:         name,
			Codec:        codec,
			MaxDoc:       maxDoc,
			DelCount:     delCount,
			SoftDelCount: softDelCount,
			IsCompound:   isCompound,
			Diagnostics:  diag,
		})
	}

	userData, _ := readMap(r)
	return summaries, userData, nil
}

// ---------- 统计报告 ----------

func buildReport(indexDir string) (*Report, error) {
	// 查找最新的 segments_N
	entries, _ := os.ReadDir(indexDir)
	var segFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), SEGMENTS_PREFIX+"_") {
			if e.Name() > segFile {
				segFile = e.Name()
			}
		}
	}
	if segFile == "" {
		return nil, errors.New("no segments found")
	}

	summaries, userData, err := parseSegmentsFile(indexDir, segFile)
	if err != nil {
		return nil, err
	}

	var totalDocs, totalDeleted, totalSoftDeleted int64
	for _, s := range summaries {
		totalDocs += int64(s.MaxDoc)
		totalDeleted += int64(s.DelCount)
		totalSoftDeleted += int64(s.SoftDelCount)
	}

	return &Report{
		IndexPath:            indexDir,
		SegmentsFile:         segFile,
		TotalSegments:        len(summaries),
		TotalDocs:            totalDocs,
		TotalDeletedDocs:     totalDeleted,
		TotalSoftDeletedDocs: totalSoftDeleted,
		UserData:             userData,
		Segments:             summaries,
	}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <index-path>")
		return
	}
	rep, err := buildReport(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	jsonOut, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(jsonOut))
}
