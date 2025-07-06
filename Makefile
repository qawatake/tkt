tape:
	@echo "Generating tapes..."
	@for tape in assets/tapes/src/*.tape; do \
		echo "Building $$tape..."; \
		vhs "$$tape"; \
	done

mod:
	go mod tidy

fmt:
	goimports -w .

install:
	go install ./cmd/tkt

maplang:
	@echo "Generating language mapping..."
	@go run ./internal/scripts/maplang adf > internal/adf/lang_map.go

prestop: mod fmt install

