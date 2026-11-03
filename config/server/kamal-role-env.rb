# Writes the per-role env files Kamal keeps on the server, for the roles that
# don't have one yet. Run by deploy.sh inside the Kamal image (as
# `ruby config/server/kamal-role-env.rb <destination> <home> <uid> <gid>`).
#
# Why: `kamal migrate` is `app exec`, which starts a one-off container with
# `--env-file ~/.kamal/apps/<service>/env/roles/<role>.env`, but only
# `kamal deploy` (app boot, start_new_version in Kamal v2.12.0's
# lib/kamal/cli/app/boot.rb) writes that file. On a host that has never booted
# this service (a new service name, a rebuilt server) the migrate step
# therefore fails before the first deploy could ever write it.
#
# This does what boot does for that file (the same Kamal code builds the
# content from the same config and secrets, so the file is byte-identical to
# the one `deploy` writes next) and nothing else: no container is started and
# no version is booted, so migrate-before-traffic is untouched. Existing
# files are left alone (the normal path), and `deploy` rewrites them anyway.
#
# The values never leave this process except into the 0600 files: nothing is
# printed and nothing goes through argv.
require "kamal"
require "fileutils"

destination, home, uid, gid = ARGV
abort "usage: kamal-role-env.rb <destination> <home> <uid> <gid>" unless gid
uid = Integer(uid)
gid = Integer(gid)

config = Kamal::Configuration.create_from(config_file: Pathname.new("config/deploy.yml"), destination: destination)

config.roles.each do |role|
  # This writes to the machine it runs on, so a role on several hosts (each
  # with its own env file) is beyond it.
  abort "#{role.name}: #{role.hosts.size} hosts; kamal-role-env.rb supports one host per role" unless role.hosts.size == 1
  host = role.primary_host
  # secrets_path is relative to the SSH user's home, as Kamal uses it.
  path = File.join(home, role.secrets_path)
  if File.exist?(path)
    puts "#{role.name}: env file present"
    next
  end

  # Create missing directories under home, owned by the deploying user (this
  # runs as root in the container; Kamal's later upload runs as that user).
  dir = File.dirname(path)
  missing = []
  d = dir
  until File.directory?(d)
    missing.unshift(d)
    d = File.dirname(d)
  end
  FileUtils.mkdir_p(dir)
  missing.each { |m| File.chown(uid, gid, m) }

  # Write then rename, so an interrupted run never leaves a partial file that
  # looks present.
  tmp = "#{path}.tmp"
  File.open(tmp, File::WRONLY | File::CREAT | File::TRUNC, 0o600) { |f| f.write(role.secrets_io(host).read) }
  File.chown(uid, gid, tmp)
  File.rename(tmp, path)
  puts "#{role.name}: env file written"
end
