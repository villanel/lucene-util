# DevOps Interview Project

## Project
After reviewing this README, see the full project brief in `project.md`.

## Getting Started

This stack runs a single-node OpenSearch plus a simple producer that writes one document per second.

```bash
docker-compose up -d
```

Notes:
- OpenSearch can take about a minute to become ready.
- Verify it by calling `http://localhost:9200`.

Index overview:

```bash
curl -s "http://localhost:9200/_cat/indices?v"
```

Search example:

```bash
curl -s "http://localhost:9200/test/_search?q=message:hello&pretty"
```

Segment overview:

```bash
curl -s "http://localhost:9200/_cat/segments?v"
```

## Find Index Data Directory by Index ID

OpenSearch stores index data under the data path. In this repo it is mapped to `./data`
from the container path `/usr/share/opensearch/data`.

1) Get the index UUID (index id):

```bash
curl -s "http://localhost:9200/_cat/indices/test?h=index,uuid"
```

2) Use the UUID to locate the on-disk directory:

```bash
ls -la ./data/nodes/0/indices/<INDEX_UUID>
```

Notes:
- The `indices/<INDEX_UUID>/` directory contains shard subfolders like `0/`, `1/`, etc.

## Index, Shard, and Segments

OpenSearch (similar to Elasticsearch) is built on top of Apache Lucene.

- Index: a logical collection of documents. On disk it maps to `indices/<INDEX_UUID>/`.
- Shard: a physical slice of an index. Each shard is a numbered directory under the index, like `indices/<INDEX_UUID>/0/`.
- Segment: an immutable Lucene segment file set inside a shard (e.g., `segments_N`, `*.si`, `*.cfs`, `*.cfe`).
  - New writes create new segments.
  - Deletes are often represented as "deleted docs" until merges reclaim them.
- A shard directory may contain extra data (e.g., translog, state), but the Lucene index directory can be detected by the presence of a `segments_N` file.

Example layout (paths are relative to `./data`):

```text
nodes/0/indices/<INDEX_UUID>/
  0/                 # shard 0
    index/           # Lucene index files
      segments_1
      _0.cfe
      _0.cfs
      _0.si
```
