import java.util.Properties

plugins {
    id("com.android.application")
    kotlin("android")
    kotlin("plugin.compose")
    kotlin("plugin.serialization")
}

// Release-Signatur aus keystore.properties im Wurzelverzeichnis (steht in
// .gitignore). Fehlt die Datei, bleibt das Release unsigniert und laesst sich
// nicht installieren - das ist gewollt. Ein Release, das stillschweigend mit
// dem Debug-Schluessel signiert wird, kann jeder faelschen: Der Schluessel
// liegt auf jedem Entwicklerrechner und hat ueberall dasselbe Passwort.
val signatur = Properties().apply {
    val datei = rootProject.file("keystore.properties")
    if (datei.exists()) datei.inputStream().use { load(it) }
}

android {
    namespace = "app.mimik"
    compileSdk = 36

    defaultConfig {
        applicationId = "app.mimik"
        minSdk = 26
        targetSdk = 36
        versionCode = 10
        versionName = "0.10"

        // Woher die App nach einer neueren Fassung fragt. Siehe Aktualisierung.kt.
        buildConfigField("String", "QUELLE", "\"merlinnolte/mimik\"")
    }
    signingConfigs {
        if (signatur.getProperty("datei") != null) {
            create("freigabe") {
                storeFile = rootProject.file(signatur.getProperty("datei"))
                storePassword = signatur.getProperty("speicherwort")
                keyAlias = signatur.getProperty("alias")
                keyPassword = signatur.getProperty("schluesselwort")
            }
        }
    }
    buildTypes {
        debug {
            // Im Debugbau steht der eigene Server voreingestellt: Wer hier
            // baut, testet gegen ihn oder gegen 10.0.2.2 und will kein Feld
            // ausfuellen.
            buildConfigField("String", "VORGABE_SERVER", "\"https://mimik.merlinnolte.de\"")
        }
        release {
            isMinifyEnabled = false
            // Keine Vorgabe: Ein oeffentliches APK darf niemanden ungefragt auf
            // einen fremden Server schicken. Wer es installiert, traegt seinen
            // eigenen ein. Speicher.wandern() sorgt dafuer, dass eine bereits
            // eingetragene Adresse dabei nicht verloren geht.
            buildConfigField("String", "VORGABE_SERVER", "\"\"")
            signingConfig = signingConfigs.findByName("freigabe")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlin { compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) } }
    buildFeatures {
        compose = true
        buildConfig = true
    }
}

// Das APK soll heissen, was es ist: mimik-0.10-release.apk statt app-release.apk.
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
