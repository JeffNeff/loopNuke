release:
	@ko resolve -f config/ > release.yaml

debug:
	@DEV=true go run ./cmd/loopNuke/main.go

image:
	@gcloud builds submit --tag gcr.io/ultra-hologram-297914/loop-nuke

staging:
	@gcloud builds submit --tag gcr.io/ultra-hologram-297914/loop-nuke-staging

apply:
	@cd config/koby && kubectl apply -f 100-registration.yaml

delete:
	@cd config/koby && kubectl delete -f 100-registration.yaml
