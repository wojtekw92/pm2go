#!/usr/bin/env bats

export PM2GO_SERVICE_PREFIX="${PM2GO_SERVICE_PREFIX:-pm2go-test-$$-}"

setup() {
    ./pm2go delete all 2>/dev/null || true
}

teardown() {
    ./pm2go delete all 2>/dev/null || true
}

@test "save and resurrect use existing systemd units" {
    run ./pm2go start test/fixtures/test-app.py --name saved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go save
    [[ "$status" -eq 0 ]]

    run ./pm2go stop saved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go resurrect
    [[ "$status" -eq 0 ]]

    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"saved-app"* ]]
    [[ "$output" == *"online"* ]]
}

@test "delete removes saved membership and resurrect never recreates a unit" {
    run ./pm2go start test/fixtures/test-app.py --name deleted-saved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go save
    [[ "$status" -eq 0 ]]

    run ./pm2go delete deleted-saved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go resurrect
    [[ "$status" -eq 0 ]]

    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" != *"deleted-saved-app"* ]]
}

@test "saving while stopped removes an application from the saved set" {
    run ./pm2go start test/fixtures/test-app.py --name unsaved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go save
    [[ "$status" -eq 0 ]]

    run ./pm2go stop unsaved-app
    [[ "$status" -eq 0 ]]

    run ./pm2go save
    [[ "$status" -eq 0 ]]

    run ./pm2go resurrect
    [[ "$status" -eq 0 ]]

    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"unsaved-app"* ]]
    [[ "$output" == *"stopped"* ]]
}
