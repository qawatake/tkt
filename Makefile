tape:
	@echo "Generating tapes..."
	@for tape in assets/tapes/src/*.tape; do \
		echo "Building $$tape..."; \
		vhs "$$tape"; \
	done

