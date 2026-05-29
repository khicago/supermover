copy_swiftpm_app_resources() {
  swiftpm_bundle_dir=$1
  app_resources_dir=$2

  if [ ! -d "$swiftpm_bundle_dir" ]; then
    return 0
  fi

  if [ -d "$swiftpm_bundle_dir/Resources" ]; then
    cp -R "$swiftpm_bundle_dir/Resources/." "$app_resources_dir/"
  fi

  for localization_dir in "$swiftpm_bundle_dir"/*.lproj; do
    [ -d "$localization_dir" ] || continue
    cp -R "$localization_dir" "$app_resources_dir/"
  done
}
