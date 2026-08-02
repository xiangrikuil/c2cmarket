#!/usr/bin/env ruby

require "yaml"

def load_workflow(path)
  source = File.read(path)
  parameters = YAML.method(:safe_load).parameters
  supports_keywords = parameters.any? do |kind, _name|
    kind == :key || kind == :keyrest
  end

  if supports_keywords
    YAML.safe_load(
      source,
      permitted_classes: [],
      permitted_symbols: [],
      aliases: true,
    )
  else
    YAML.safe_load(source, [], [], true)
  end
end

repo_root = File.expand_path("..", __dir__)
release_workflow = load_workflow(
  File.join(repo_root, ".github/workflows/release-backend.yml"),
)
ci_workflow = load_workflow(
  File.join(repo_root, ".github/workflows/ci.yml"),
)

failures = []
steps = release_workflow.dig("jobs", "publish", "steps") || []
metadata_step = steps.find { |step| step["id"] == "metadata" }
build_step = steps.find do |step|
  step.fetch("uses", "").start_with?("docker/build-push-action@")
end

if metadata_step.nil?
  failures << "release workflow must resolve build metadata in a metadata step"
else
  unless metadata_step.dig("env", "GIT_SHA") == "${{ inputs.git_sha }}"
    failures << "metadata step must resolve the workflow git_sha input"
  end

  metadata_script = metadata_step.fetch("run", "")
  unless metadata_script.include?('git show -s --format=%cI "${GIT_SHA}"')
    failures << "metadata step must derive build time from the release commit"
  end
  unless metadata_script.include?('"build_time=${build_time}" >>"${GITHUB_OUTPUT}"')
    failures << "metadata step must expose build_time as an output"
  end
end

if build_step.nil?
  failures << "release workflow must use docker/build-push-action"
else
  build_args = build_step.dig("with", "build-args").to_s.lines.map(&:strip)
  required_build_args = [
    "APP_VERSION=${{ inputs.release_tag }}",
    "GIT_COMMIT=${{ inputs.git_sha }}",
    "BUILD_TIME=${{ steps.metadata.outputs.build_time }}",
  ]
  required_build_args.each do |value|
    failures << "release build args are missing #{value}" unless build_args.include?(value)
  end

  labels = build_step.dig("with", "labels").to_s.lines.map(&:strip)
  required_labels = [
    "org.opencontainers.image.version=${{ inputs.release_tag }}",
    "org.opencontainers.image.revision=${{ inputs.git_sha }}",
    "org.opencontainers.image.created=${{ steps.metadata.outputs.build_time }}",
  ]
  required_labels.each do |value|
    failures << "release image labels are missing #{value}" unless labels.include?(value)
  end
end

ci_steps = ci_workflow.dig("jobs", "contracts", "steps") || []
unless ci_steps.any? { |step| step["run"] == "ruby scripts/check-release-workflow.rb" }
  failures << "contracts job must run the release workflow contract check"
end

tailscale_action = "tailscale/github-action@780049a30b6ff5c378a9e7b389d15ece7a204888"
%w[deploy-staging deploy-production].each do |job_name|
  deploy_steps = ci_workflow.dig("jobs", job_name, "steps") || []
  tailscale_index = deploy_steps.index do |step|
    step["uses"] == tailscale_action
  end
  ssh_identity_index = deploy_steps.index do |step|
    step["name"] == "Configure deployment SSH identity"
  end

  if tailscale_index.nil?
    failures << "#{job_name} must connect through the pinned Tailscale action"
    next
  end

  tailscale_step = deploy_steps[tailscale_index]
  required_inputs = {
    "oauth-client-id" => "${{ secrets.TS_OAUTH_CLIENT_ID }}",
    "oauth-secret" => "${{ secrets.TS_OAUTH_SECRET }}",
    "tags" => "tag:c2c-ci",
    "ping" => "${{ secrets.VPS_HOST }}",
    "version" => "1.98.10",
  }
  required_inputs.each do |name, value|
    unless tailscale_step.dig("with", name) == value
      failures << "#{job_name} Tailscale input #{name} must be #{value}"
    end
  end

  if !ssh_identity_index.nil? && tailscale_index > ssh_identity_index
    failures << "#{job_name} must join Tailscale before configuring SSH"
  end
end

unless failures.empty?
  warn failures.join("\n")
  exit 1
end

puts "Release workflow contract passed."
