plugins {
    id("com.android.application")
    kotlin("android")
    kotlin("plugin.compose")
    kotlin("plugin.serialization")
}

android {
    namespace = "app.mimik"
    compileSdk = 36

    defaultConfig {
        applicationId = "app.mimik"
        minSdk = 26
        targetSdk = 36
        versionCode = 8
        versionName = "0.8"
    }
    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlin { compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) } }
    buildFeatures { compose = true }
}

// Das APK soll heissen, was es ist: mimik-0.6-debug.apk statt app-debug.apk.
// Beim Herunterladen aus einem Release liegen sonst drei Dateien gleichen
// Namens nebeneinander, und keine sagt, welche Fassung sie ist.
base { archivesName.set("mimik-" + android.defaultConfig.versionName) }

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation(platform("androidx.compose:compose-bom:2024.12.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui-tooling-preview")
    debugImplementation("androidx.compose.ui:ui-tooling")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    // Benachrichtigungen ohne Push-Dienst: ein wiederkehrender Auftrag, der
    // selbst beim Server nachfragt. Siehe Melder.kt.
    implementation("androidx.work:work-runtime-ktx:2.10.0")
    testImplementation("junit:junit:4.13.2")
}
