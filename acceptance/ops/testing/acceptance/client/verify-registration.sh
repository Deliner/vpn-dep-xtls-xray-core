#!/usr/bin/env bash
# Registration and ordering are distinct native tests, bound to their input binaries.
client_verify_registration() {
  local kind="$1" mode="$2" test_name
  case "$kind" in
    install) test_name=TestKernelClientServiceInstallation ;;
    boot-order) test_name=TestKernelBootOrderingFailsClosed ;;
    *) return 2 ;;
  esac
  awk -v name="$test_name" 'index($0,"--- PASS: " name " (") { root++ }
    /^PASS$/ { complete++ } /--- (FAIL|SKIP):/ || /^FAIL$/ || /WARNING: DATA RACE/ { bad=1 }
    END { exit bad || root != 1 || complete != 1 }' "$mode/guest.log" || return 1
  if [[ "$kind" == install ]]; then
    client_verify_registration_lifecycle "$mode/guest.log" || return 1
  else
    client_verify_registration_order "$mode/guest.log" || return 1
  fi
}

client_verify_registration_lifecycle() {
  awk '
    /actual CLI service-enable success=false: supported network managers:/ { unsupported++ }
    /actual CLI service-enable success=true:/ { enabled++ }
    /actual CLI service-remove success=false: client boot registration is retained;/ { refused++ }
    /actual CLI service-disable success=true:/ { disabled++ }
    /boot registration verified: disabled by default, enable\/idempotent\/dependencies\/unregister refusal\/disable; no service activation/ { boot++ }
    /installation verified: initial\/idempotent\/update\/rollback\/interrupted reload\/remove; generations/ {
      lifecycle++; value=$0; sub(/^.*generations /,"",value); split(value,fields," ")
      if (length(fields[1])!=64 || fields[1] ~ /[^0-9a-f]/ || length(fields[2])!=64 || fields[2] ~ /[^0-9a-f]/ || fields[1]==fields[2] || fields[3]!="retained") bad=1
    }
    END { exit bad || unsupported!=1 || enabled!=2 || refused!=1 || disabled!=2 || boot!=1 || lifecycle!=1 }
  ' "$1" || return 1
}

client_verify_registration_order() {
  awk '
    /control: Wants\+After permits dependent activation after protection failure/ { control++ }
    /required protection: failure blocks dependent activation/ { failure++ }
    /native systemd verified: successful protection precedes dependent; unit stop retains firewall; explicit client stop restores connectivity/ { retained++ }
    END { exit control!=1 || failure!=1 || retained!=1 }
  ' "$1" || return 1
}
