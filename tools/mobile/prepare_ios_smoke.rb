# Adds a temporary XCTest UI target to the ignored Expo-generated project.
# The Python runner restores the original project and scheme when it finishes.
require 'json'
require 'pathname'
require 'xcodeproj'

warn "Ruby #{RUBY_VERSION}: #{RbConfig.ruby}; xcodeproj #{Xcodeproj::VERSION}"
abort 'Usage: prepare_ios_smoke.rb <project.xcodeproj> <config.json>' unless ARGV.length == 2
project_path, config_path = ARGV.map { |value| File.expand_path(value) }
config = JSON.parse(File.read(config_path))
project = Xcodeproj::Project.open(project_path)
application = project.targets.find { |target| target.product_type == 'com.apple.product-type.application' }
abort 'Generated project has no application target' unless application
name = 'Mig33SmokeUITests'
abort "Target #{name} already exists; restore the generated project before running" if project.targets.any? { |target| target.name == name }

target = project.new_target(:ui_test_bundle, name, :ios, '16.4')
target.add_dependency(application)
source = project.main_group.new_file(File.expand_path('Mig33SmokeTests.swift', __dir__))
target.source_build_phase.add_file_reference(source)
target.build_configurations.each do |build|
  build.build_settings.merge!({
    'PRODUCT_BUNDLE_IDENTIFIER' => 'com.hailam.mig33.smoke',
    'PRODUCT_NAME' => '$(TARGET_NAME)',
    'SWIFT_VERSION' => '5.0',
    'GENERATE_INFOPLIST_FILE' => 'YES',
    'TARGETED_DEVICE_FAMILY' => '1,2',
    'TEST_TARGET_NAME' => application.name,
    'ENABLE_TESTING_SEARCH_PATHS' => 'YES',
    # Simulator signing is ad-hoc; disabling signing strips the entitlements
    # SecureStore needs to exercise the real iOS Keychain.
    'CODE_SIGNING_ALLOWED' => 'YES',
    'CODE_SIGN_IDENTITY[sdk=iphonesimulator*]' => '-',
    'FRAMEWORK_SEARCH_PATHS' => ['$(inherited)', '$(PLATFORM_DIR)/Developer/Library/Frameworks'],
    'LIBRARY_SEARCH_PATHS' => ['$(inherited)', '$(PLATFORM_DIR)/Developer/usr/lib'],
  })
end
attributes = project.root_object.attributes['TargetAttributes'] ||= {}
attributes[target.uuid] = { 'CreatedOnToolsVersion' => '27.2', 'TestTargetID' => application.uuid }
project.save

scheme = Xcodeproj::XCScheme.new
scheme.add_build_target(application)
scheme.add_build_target(target, false)
scheme.add_test_target(target)
scheme.set_launch_target(application)
scheme.test_action.should_use_launch_scheme_args_env = false
scheme.test_action.environment_variables = Xcodeproj::XCScheme::EnvironmentVariables.new(
  config.fetch('test_environment').map { |key, value| { key: key, value: value.to_s, enabled: true } }
)
scheme.save_as(project_path, name, true)
puts "Prepared #{name} in ignored #{project_path}"
