# Include custom targets and environment variables here

E2E_DIR = e2e/playwright

$(E2E_DIR)/node_modules: $(E2E_DIR)/package-lock.json
	cd $(E2E_DIR) && $(NPM) ci && npx playwright install chromium
	touch $@

.PHONY: e2e-env-up
e2e-env-up: $(E2E_DIR)/node_modules ## Starts the Mattermost stack used by the Playwright suite
	cd $(E2E_DIR) && npm run env:up

.PHONY: e2e-env-down
e2e-env-down: ## Stops the Playwright Mattermost stack and drops its data
	cd $(E2E_DIR) && npm run env:down

.PHONY: e2e
e2e: dist $(E2E_DIR)/node_modules ## Runs the Playwright suite against the stack from e2e-env-up
	cd $(E2E_DIR) && npx playwright test
