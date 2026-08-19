.PHONY: diff-univers harvest

diff-univers:
	go run ./tools/diff-univers $(DIFF_UNIVERS_ARGS)

harvest:
	go run ./tools/harvest $(HARVEST_ARGS)
