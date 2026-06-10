#!/bin/sh
# Removes the Apple developer team ID and bundle identifier from the Xcode
# project before committing, so neither your team nor a fixed bundle ID is
# published. After cloning, set your own in Xcode: Signing & Capabilities >
# Team, and change the Bundle Identifier to a unique string.
PBXPROJ=whitelist-bypass-proxy.xcodeproj/project.pbxproj
sed -i '' 's/DEVELOPMENT_TEAM = .*;/DEVELOPMENT_TEAM = "";/' "$PBXPROJ"
sed -i '' 's/PRODUCT_BUNDLE_IDENTIFIER = .*;/PRODUCT_BUNDLE_IDENTIFIER = "com.example.whitelist-bypass-proxy";/' "$PBXPROJ"
echo "Stripped DEVELOPMENT_TEAM and PRODUCT_BUNDLE_IDENTIFIER from project.pbxproj"
