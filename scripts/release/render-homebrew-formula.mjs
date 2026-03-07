#!/usr/bin/env node

import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const owner = 'lteawoo';
const repo = 'Cohesion';
const defaultOutputPath = path.resolve('packaging/homebrew/Formula/cohesion.rb');

function parseArgs(argv) {
  const parsed = {
    tag: '',
    outputPath: defaultOutputPath,
  };

  for (let index = 2; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === '--tag') {
      parsed.tag = argv[index + 1] ?? '';
      index += 1;
      continue;
    }
    if (arg === '--output') {
      parsed.outputPath = path.resolve(argv[index + 1] ?? defaultOutputPath);
      index += 1;
      continue;
    }
    throw new Error(`unknown argument: ${arg}`);
  }

  return parsed;
}

function normalizeDigest(asset) {
  if (typeof asset.digest !== 'string') {
    return '';
  }
  if (!asset.digest.startsWith('sha256:')) {
    return '';
  }
  return asset.digest.slice('sha256:'.length).trim();
}

function assetMapFromRelease(release) {
  const assets = new Map();
  for (const asset of release.assets ?? []) {
    const sha256 = normalizeDigest(asset);
    if (!asset?.name || !asset?.browser_download_url || !sha256) {
      continue;
    }
    assets.set(asset.name, {
      url: asset.browser_download_url,
      sha256,
    });
  }
  return assets;
}

function requireAsset(assets, name) {
  const asset = assets.get(name);
  if (!asset) {
    throw new Error(`required release asset not found: ${name}`);
  }
  return asset;
}

function renderFormula({ version, tag, darwinAmd64, darwinArm64, linuxAmd64, linuxArm64 }) {
  return `class Cohesion < Formula
  desc "Self-hosted file service for browsing and sharing files"
  homepage "https://github.com/${owner}/${repo}"
  version "${version}"
  license "AGPL-3.0-only"

  on_macos do
    if Hardware::CPU.arm?
      url "${darwinArm64.url}"
      sha256 "${darwinArm64.sha256}"
    else
      url "${darwinAmd64.url}"
      sha256 "${darwinAmd64.sha256}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${linuxArm64.url}"
      sha256 "${linuxArm64.sha256}"
    else
      url "${linuxAmd64.url}"
      sha256 "${linuxAmd64.sha256}"
    end
  end

  def install
    bin.install "cohesion"
    bin.install "cohesion-updater"
    pkgshare.install "README.md"
    pkgshare.install "config/config.prod.yaml"
  end

  def post_install
    runtime_dir = var/"cohesion/runtime"

    (runtime_dir/"data").mkpath
    (runtime_dir/"logs").mkpath
  end

  service do
    run [opt_bin/"cohesion"]
    environment_variables COHESION_RUNTIME_ROOT: (var/"cohesion/runtime").to_s
    working_dir (var/"cohesion").to_s
    keep_alive true
    log_path var/"log/cohesion.log"
    error_log_path var/"log/cohesion.error.log"
  end

  def caveats
    <<~EOS
      Cohesion config path:
        ~/.cohesion/config/config.prod.yaml

      Cohesion data path:
        ~/.cohesion/data/cohesion.db

      Cohesion secrets path:
        ~/.cohesion/secrets/

      Start the service:
        brew services start cohesion

      Stop the service:
        brew services stop cohesion

      Update the package:
        brew upgrade cohesion

      In-app self-update is disabled for Homebrew installs.
    EOS
  end

  test do
    assert_predicate bin/"cohesion", :exist?
    assert_predicate bin/"cohesion-updater", :exist?
    assert_predicate pkgshare/"config.prod.yaml", :exist?
  end
end
`;
}

async function fetchRelease(tag) {
  const endpoint = tag
    ? `https://api.github.com/repos/${owner}/${repo}/releases/tags/${encodeURIComponent(tag)}`
    : `https://api.github.com/repos/${owner}/${repo}/releases/latest`;

  const response = await fetch(endpoint, {
    headers: {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'cohesion-homebrew-formula-renderer',
    },
  });
  if (!response.ok) {
    throw new Error(`release lookup failed with status ${response.status}`);
  }
  return response.json();
}

async function main() {
  const { tag, outputPath } = parseArgs(process.argv);
  const release = await fetchRelease(tag);
  const releaseTag = String(release.tag_name ?? '').trim();
  const version = releaseTag.replace(/^v/i, '');
  if (!releaseTag || !version) {
    throw new Error('release tag is empty');
  }

  const assets = assetMapFromRelease(release);
  const formula = renderFormula({
    version,
    tag: releaseTag,
    darwinAmd64: requireAsset(assets, `cohesion_${version}_apple_darwin_amd64.tar.gz`),
    darwinArm64: requireAsset(assets, `cohesion_${version}_apple_darwin_arm64.tar.gz`),
    linuxAmd64: requireAsset(assets, `cohesion_${version}_linux_amd64.tar.gz`),
    linuxArm64: requireAsset(assets, `cohesion_${version}_linux_arm64.tar.gz`),
  });

  await mkdir(path.dirname(outputPath), { recursive: true });
  await writeFile(outputPath, formula, 'utf8');
  process.stdout.write(`rendered ${releaseTag} formula to ${outputPath}\n`);
}

main().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
