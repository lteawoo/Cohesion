class Cohesion < Formula
  desc "Self-hosted file service for browsing and sharing files"
  homepage "https://github.com/lteawoo/Cohesion"
  version "0.5.18"
  license "AGPL-3.0-only"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.18/cohesion_0.5.18_apple_darwin_arm64.tar.gz"
      sha256 "407595d319dd135e26b57ecacec099dc4b46df2befe2cbc8beeff5e93f7e2a97"
    else
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.18/cohesion_0.5.18_apple_darwin_amd64.tar.gz"
      sha256 "257f4763c8246276f90d40d09ba22a28f7551e2d6c38525736a1bc34005040fc"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.18/cohesion_0.5.18_linux_arm64.tar.gz"
      sha256 "038f46732da62acc48e50b021f8c1733b604e4f70608670b5b6d886550270bff"
    else
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.18/cohesion_0.5.18_linux_amd64.tar.gz"
      sha256 "b58e142a826af13d3aea92895707bc37188f0fb9a9a2127e4ec21237931210db"
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
