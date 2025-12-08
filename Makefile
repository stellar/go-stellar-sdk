# Docker build targets use an optional "TAG" environment
# variable can be set to use custom tag name. For example:
#   TAG=my-registry.example.com/keystore:dev make keystore
XDRS = xdr/Stellar-SCP.x \
xdr/Stellar-ledger-entries.x \
xdr/Stellar-ledger.x \
xdr/Stellar-overlay.x \
xdr/Stellar-transaction.x \
xdr/Stellar-types.x \
xdr/Stellar-contract-env-meta.x \
xdr/Stellar-contract-meta.x \
xdr/Stellar-contract-spec.x \
xdr/Stellar-contract.x \
xdr/Stellar-internal.x \
xdr/Stellar-contract-config-setting.x \
xdr/Stellar-exporter.x


XDRGEN_COMMIT=af107f07237a15fcf5f9aea71b2bebfcc1113b45
XDR_COMMIT=0a621ec7811db000a60efae5b35f78dee3aa2533

.PHONY: xdr xdr-clean xdr-update

keystore:
	$(MAKE) -C services/keystore/ docker-build

ticker:
	$(MAKE) -C services/ticker/ docker-build

horizon:
	$(MAKE) -C services/horizon/ binary-build

galexie:
	$(MAKE) -C services/galexie/ docker-build

webauth:
	$(MAKE) -C exp/services/webauth/ docker-build

recoverysigner:
	$(MAKE) -C exp/services/recoverysigner/ docker-build

regulated-assets-approval-server:
	$(MAKE) -C services/regulated-assets-approval-server/ docker-build

gxdr/xdr_generated.go: $(XDRS)
	go run github.com/xdrpp/goxdr/cmd/goxdr -p gxdr -enum-comments -o $@ $(XDRS)
	gofmt -s -w $@

xdr/%.x:
	printf "%s" ${XDR_COMMIT} > xdr/xdr_commit_generated.txt
	curl -Lsf -o $@ https://raw.githubusercontent.com/stellar/stellar-xdr/$(XDR_COMMIT)/$(@F)

xdr/xdr_generated.go: $(XDRS)
	docker run -it --rm -v $$PWD:/wd -w /wd ruby /bin/bash -c '\
		gem install specific_install -v 0.3.8 && \
		gem specific_install https://github.com/stellar/xdrgen.git -b $(XDRGEN_COMMIT) && \
		xdrgen \
			--language go \
			--namespace xdr \
			--output xdr/ \
			$(XDRS)'
	# No, you're not reading the following wrong. Apperantly, running gofmt twice required to complete it's formatting.
	gofmt -s -w $@
	gofmt -s -w $@

xdr: gxdr/xdr_generated.go xdr/xdr_generated.go

xdr-clean:
	rm $(XDRS) || true

xdr-update: xdr-clean xdr


##############
PROTO_DIR := protos
GEN_SUFFIX := .pb.go
PROTO_FILES := $(shell find $(PROTO_DIR) -name "*.proto")
PROTO_CHECKSUM := .proto_checksums

generate-proto:
	@echo "Regenerating proto files..."
	@touch $(PROTO_CHECKSUM)
	@current_checksum=$$(sha256sum $(PROTO_FILES) | sha256sum | awk '{print $$1}'); \
	stored_checksum=$$(cat $(PROTO_CHECKSUM)); \
	if [ "$${current_checksum}" != "$${stored_checksum}" ]; then \
    	echo "Changes detected. Regenerating all proto files..."; \
    	MAP_OPTS=$$(for file in $(PROTO_FILES); do \
    		rel_path=$$(echo $$file | sed 's|$(PROTO_DIR)/||'); \
    		pkg_path=$$(dirname $$rel_path); \
    		go_pkg="github.com/stellar/go-stellar-sdk/$$pkg_path"; \
    		printf "M%s=%s," "$$rel_path" "$$go_pkg"; \
    	done); \
    	MAP_OPTS=$${MAP_OPTS%,}; \
    	echo "Running protoc with options:"; \
    	echo "  --go_out=."; \
    	echo "  --go_opt=paths=source_relative"; \
    	echo "  --go_opt=$$MAP_OPTS"; \
    	echo "Proto Files:  $(PROTO_FILES)"; \
    	protoc -I=$(PROTO_DIR) \
    	       --go_out=. --go_opt=paths=source_relative \
    	       --go_opt=$$MAP_OPTS \
    	       $(PROTO_FILES); \
    	echo "$${current_checksum}" > $(PROTO_CHECKSUM); \
    else \
    	echo "No changes detected in proto files."; \
    fi


regenerate-proto: $(PROTO_CHECKSUM)
	rm -f $(PROTO_CHECKSUM)
	$(MAKE) generate-proto

$(PROTO_CHECKSUM):
	@touch $(PROTO_CHECKSUM)

.PHONY: generate-proto regenerate-proto
