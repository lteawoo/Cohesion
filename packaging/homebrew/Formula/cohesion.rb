class Cohesion < Formula
  desc "Self-hosted file service for browsing and sharing files"
  homepage "https://github.com/lteawoo/Cohesion"
  version "0.5.17"
  license "AGPL-3.0-only"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.17/cohesion_0.5.17_apple_darwin_arm64.tar.gz"
      sha256 "2f90a94730606776eaeea5b51012ca6e3c8e44d47bffdbdffc168baac1d9950f"
    else
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.17/cohesion_0.5.17_apple_darwin_amd64.tar.gz"
      sha256 "6452826e7099fe680db2c369c5e1e0b58df8f8b2ef3ab4dd03c366ad17ea9e96"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.17/cohesion_0.5.17_linux_arm64.tar.gz"
      sha256 "c3940916ad98d443f8ebe03000c91d238f80de9fc21759d7a4abc6b3d24b61ec"
    else
      url "https://github.com/lteawoo/Cohesion/releases/download/v0.5.17/cohesion_0.5.17_linux_amd64.tar.gz"
      sha256 "93b3f465269c3d9aa785f90bfb37cf7091efb3f2e4e2bbe83d19ffde532afac1"
    end
  end

  def install
    bin.install "cohesion"
    bin.install "cohesion-updater"
    pkgshare.install "README.md"
    pkgshare.install "config/config.prod.yaml"
  end

  def post_install
    config_dir = var/"cohesion/config"
    runtime_dir = var/"cohesion/runtime"

    config_dir.mkpath
    (config_dir/"data").mkpath
    (runtime_dir/"data").mkpath
    (runtime_dir/"logs").mkpath

    config_path = config_dir/"config.prod.yaml"
    config_path.write((pkgshare/"config.prod.yaml").read) unless config_path.exist?
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
        #{var}/cohesion/config/config.prod.yaml

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
