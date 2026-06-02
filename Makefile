DUCKDB ?= duckdb
GO ?= go
FLATC ?= flatc

VERSION ?= 2025-04-15.0
SOURCE ?= overturemaps-2025-04-15

EXTRACT_SQL ?= tools/extract.sql
BUILD_DIR ?= build
DIST_DIR ?= dist
DIST_STAMP := $(DIST_DIR)/.dir
EXTRACTED ?= $(BUILD_DIR)/extracted.parquet
SIMPLIFIED ?= $(BUILD_DIR)/simplified.parquet
TOPO_SIMPLIFIED ?= $(BUILD_DIR)/topo-simplified.parquet
DIVISIONS_BIN ?= $(BUILD_DIR)/divisions.bin
DIVISIONS_CF_BIN ?= $(BUILD_DIR)/divisions.cf.bin
REMOTE_INDEX ?= $(BUILD_DIR)/divisions.xs-index.gz
REMOTE_SLAB  ?= $(BUILD_DIR)/divisions.xs-poly
SCHEMA ?= schema/xiangshan.fbs
COMPRESSED_SCHEMA ?= schema/xiangshan_compressed.fbs
PUBLIC_CMD_TOOLS := xs-query xs-serve
PUBLIC_CMD_BINS := $(addprefix $(DIST_DIR)/,$(PUBLIC_CMD_TOOLS))

.PHONY: all pipeline extract simplify topo-simplify encode compress remote-split dist build-cmds generate test bench query serve verify stats stats-extract stats-simplify stats-topo-simplify stats-bin clean dep-licenses help FORCE

all: pipeline

pipeline: extract simplify encode compress

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(DIST_STAMP):
	mkdir -p $(DIST_DIR)
	touch $(DIST_STAMP)

extract: $(EXTRACTED)

$(EXTRACTED): $(EXTRACT_SQL) | $(BUILD_DIR)
	$(DUCKDB) -c "$$(cat $(EXTRACT_SQL))"

simplify: $(SIMPLIFIED)

$(SIMPLIFIED): $(EXTRACTED)
	$(GO) run ./internal/cmd/xs-simplify -input $(EXTRACTED) -output $(SIMPLIFIED)

topo-simplify: $(TOPO_SIMPLIFIED)

# Experimental and intentionally not part of the default pipeline.
$(TOPO_SIMPLIFIED): $(EXTRACTED)
	$(GO) run ./internal/cmd/xs-topo-simplify -input $(EXTRACTED) -output $(TOPO_SIMPLIFIED)

encode: $(DIVISIONS_BIN)

$(DIVISIONS_BIN): $(SIMPLIFIED)
	$(GO) run ./internal/cmd/xs-encode -input $(SIMPLIFIED) -output $(DIVISIONS_BIN) -version "$(VERSION)" -source "$(SOURCE)"

compress: $(DIVISIONS_CF_BIN)

$(DIVISIONS_CF_BIN): $(DIVISIONS_BIN)
	$(GO) run ./internal/cmd/xs-compress -input $(DIVISIONS_BIN) -output $(DIVISIONS_CF_BIN)

remote-split: $(REMOTE_INDEX)

$(REMOTE_INDEX): $(DIVISIONS_CF_BIN)
	$(GO) run ./internal/cmd/xs-remote-split \
		-input $(DIVISIONS_CF_BIN) \
		-index $(REMOTE_INDEX) \
		-slab $(REMOTE_SLAB) \
		--compress

dist: build-cmds

build-cmds: $(PUBLIC_CMD_BINS)

$(DIST_DIR)/%: FORCE cmd/%/*.go go.mod go.sum | $(DIST_STAMP)
	$(GO) build -o $@ ./cmd/$*

generate: $(SCHEMA) $(COMPRESSED_SCHEMA)
	$(FLATC) --go -o generated $(SCHEMA)
	$(FLATC) --go -o generated $(COMPRESSED_SCHEMA)
	gofmt -w generated/xiangshan/v1

test:
	$(GO) test ./...

bench:
	$(GO) test -bench=. -benchmem ./...

query: $(DIVISIONS_CF_BIN)
	$(GO) run ./cmd/xs-query -data $(DIVISIONS_CF_BIN) -lng "$${LNG:?set LNG}" -lat "$${LAT:?set LAT}"

serve: $(DIVISIONS_CF_BIN)
	$(GO) run ./cmd/xs-serve -data $(DIVISIONS_CF_BIN) -addr "$${ADDR:-:8080}"

verify: stats test

stats: stats-extract stats-bin

stats-extract: $(EXTRACTED)
	$(DUCKDB) -c "SELECT count(*) AS rows_total, count(*) FILTER (WHERE name IS NOT NULL) AS named, count(*) FILTER (WHERE geometry_wkb IS NOT NULL) AS geometry_rows, count(*) FILTER (WHERE subtype='country') AS country, count(*) FILTER (WHERE subtype IN ('region','macroregion')) AS regions, count(*) FILTER (WHERE subtype IN ('county','macrocounty')) AS counties, count(*) FILTER (WHERE country IS NULL) AS country_null FROM read_parquet('$(EXTRACTED)');"

stats-simplify: $(SIMPLIFIED)
	$(DUCKDB) -c "INSTALL spatial; LOAD spatial; SELECT subtype, count(*) AS rows, sum(ST_NPoints(ST_GeomFromWKB(geometry_wkb))) AS points FROM read_parquet('$(SIMPLIFIED)') GROUP BY subtype ORDER BY subtype;"

stats-topo-simplify: $(TOPO_SIMPLIFIED)
	$(DUCKDB) -c "INSTALL spatial; LOAD spatial; SELECT subtype, count(*) AS rows, sum(ST_NPoints(ST_GeomFromWKB(geometry_wkb))) AS points FROM read_parquet('$(TOPO_SIMPLIFIED)') GROUP BY subtype ORDER BY subtype;"

stats-bin: $(DIVISIONS_BIN) $(DIVISIONS_CF_BIN)
	ls -lh $(EXTRACTED) $(DIVISIONS_BIN) $(DIVISIONS_CF_BIN)
	@if [ -f $(REMOTE_INDEX) ]; then ls -lh $(REMOTE_INDEX) $(REMOTE_SLAB); fi
	python3 -c 'from pathlib import Path; p = Path("$(DIVISIONS_BIN)"); b = p.read_bytes()[:12]; print("divisions_bin_bytes", p.stat().st_size); print("size_prefix_hex", b[:4].hex()); print("identifier", b[8:12].decode("ascii", "replace"))'

clean:
	rm -rf $(BUILD_DIR)

dep-licenses:
	rm -rf THIRD_PARTY_LICENSES
	$(GO) run github.com/google/go-licenses/v2@latest save ./... --save_path=THIRD_PARTY_LICENSES --force
	bash build_notice.sh


help:
	@echo "Targets:"
	@echo "  make pipeline       Run extract, simplify, encode, compress"
	@echo "  make extract        Build $(EXTRACTED)"
	@echo "  make simplify       Build $(SIMPLIFIED) via topology-aware simplification (xs-simplify)"
	@echo "  make topo-simplify  Build $(TOPO_SIMPLIFIED)"
	@echo "  make encode         Build $(DIVISIONS_BIN) from $(SIMPLIFIED)"
	@echo "  make compress       Build $(DIVISIONS_CF_BIN)"
	@echo "  make remote-split   Build $(REMOTE_INDEX) + $(REMOTE_SLAB) for remote finder"
	@echo "  make dist           Build public cmd tools into $(DIST_DIR)"
	@echo "  make stats          Print extract and binary stats"
	@echo "  make test           Run go test ./..."
	@echo "  make bench          Run go test -bench=. -benchmem ./..."
	@echo "  make query          Run xs-query with LNG and LAT"
	@echo "  make serve          Run xs-serve with optional ADDR"
	@echo "  make generate       Regenerate FlatBuffers Go code"
	@echo "  make clean          Remove $(BUILD_DIR)"
	@echo "  make dep-licenses   Generate third-party licenses"
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  SOURCE=$(SOURCE)"
