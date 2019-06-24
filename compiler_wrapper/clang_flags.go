package main

import (
	"bytes"
	"path/filepath"
	"strings"
)

func processClangFlags(rootPath string, builder *commandBuilder) error {
	env := builder.env
	clangDir := env.getenv("CLANG")

	if clangDir == "" {
		clangDir = filepath.Join(rootPath, "usr/bin/")
		if !filepath.IsAbs(builder.path) {
			// If sysroot_wrapper is invoked by relative path, call actual compiler in
			// relative form. This is neccesary to remove absolute path from compile
			// outputs.
			var err error
			clangDir, err = filepath.Rel(env.getwd(), clangDir)
			if err != nil {
				return wrapErrorwithSourceLocf(err, "failed to make clangDir %s relative to %s.", clangDir, env.getwd())
			}
		}
	} else {
		clangDir = filepath.Dir(clangDir)
	}

	// GCC flags to remove from the clang command line.
	// TODO: Once clang supports GCC compatibility mode, remove
	// these checks.
	//
	// Use of -Qunused-arguments allows this set to be small, just those
	// that clang still warns about.
	unsupported := map[string]bool{
		"-mno-movbe":                  true,
		"-pass-exit-codes":            true,
		"-Wclobbered":                 true,
		"-Wno-psabi":                  true,
		"-Wlogical-op":                true,
		"-Wmissing-parameter-type":    true,
		"-Wold-style-declaration":     true,
		"-Woverride-init":             true,
		"-Wunsafe-loop-optimizations": true,
	}

	unsupportedPrefixes := []string{"-Wstrict-aliasing=", "-finline-limit="}

	// clang with '-ftrapv' generates 'call __mulodi4', which is only implemented
	// in compiler-rt library. However compiler-rt library only has i386/x86_64
	// backends (see '/usr/lib/clang/3.7.0/lib/linux/libclang_rt.*'). GCC, on the
	// other hand, generate 'call __mulvdi3', which is implemented in libgcc. See
	// bug chromium:503229.
	armUnsupported := map[string]bool{"-ftrapv": true}

	// Clang may use different options for the same or similar functionality.
	gccToClang := map[string]string{
		"-Wno-error=cpp":                     "-Wno-#warnings",
		"-Wno-error=maybe-uninitialized":     "-Wno-error=uninitialized",
		"-Wno-error=unused-but-set-variable": "-Wno-error=unused-variable",
		"-Wno-unused-but-set-variable":       "-Wno-unused-variable",
		"-Wunused-but-set-variable":          "-Wunused-variable",
	}

	// Note: not using builder.transformArgs as we need to add multiple arguments
	// based on a single input argument, and also be able to return errors.
	newArgs := []builderArg{}

	for _, arg := range builder.args {
		// Adds an argument with the given value, preserving the
		// fromUser value of the original argument.
		addNewArg := func(value string) {
			newArgs = append(newArgs, builderArg{
				fromUser: arg.fromUser,
				value:    value,
			})
		}

		if mapped, ok := gccToClang[arg.value]; ok {
			addNewArg(mapped)
			continue
		}

		if unsupported[arg.value] {
			continue
		}

		if hasAtLeastOnePrefix(arg.value, unsupportedPrefixes) {
			continue
		}

		if builder.target.arch == "armv7a" && builder.target.sys == "linux" {
			if armUnsupported[arg.value] {
				continue
			}
		}

		if clangOnly := "-Xclang-only="; strings.HasPrefix(arg.value, clangOnly) {
			addNewArg(arg.value[len(clangOnly):])
			continue
		}

		if clangPath := "-Xclang-path="; strings.HasPrefix(arg.value, clangPath) {
			readResourceCmd := &command{
				path: filepath.Join(clangDir, builder.target.compiler),
				args: []string{"--print-resource-dir"},
			}
			stdoutBuffer := bytes.Buffer{}
			if err := env.run(readResourceCmd, &stdoutBuffer, env.stderr()); err != nil {
				return wrapErrorwithSourceLocf(err,
					"failed to call clang to read the resouce-dir: %s %s",
					readResourceCmd.path, readResourceCmd.args)
			}

			clangPathValue := arg.value[len(clangPath):]
			clangDir = clangPathValue

			addNewArg("-resource-dir=" + stdoutBuffer.String())
			addNewArg("--gcc-toolchain=/usr")
			continue
		}

		addNewArg(arg.value)
	}
	builder.args = newArgs

	builder.path = filepath.Join(clangDir, builder.target.compiler)

	// Specify the target for clang.
	linkerPath := getLinkerPath(env, builder.target.target+"-ld", rootPath)
	relLinkerPath, err := filepath.Rel(env.getwd(), linkerPath)
	if err != nil {
		return wrapErrorwithSourceLocf(err, "failed to make linker path %s relative to %s",
			linkerPath, env.getwd())
	}
	builder.addPostUserArgs("-B" + relLinkerPath)
	if startswithI86(builder.target.arch) {
		// TODO: -target i686-pc-linux-gnu causes clang to search for
		// libclang_rt.asan-i686.a which doesn't exist because it's packaged
		// as libclang_rt.asan-i386.a. We can't use -target i386-pc-linux-gnu
		// because then it would try to run i386-pc-linux-gnu-ld which doesn't
		// exist. Consider renaming the runtime library to use i686 in its name.
		builder.addPostUserArgs("-m32")
		// clang does not support -mno-movbe. This is the alternate way to do it.
		builder.addPostUserArgs("-Xclang", "-target-feature", "-Xclang", "-movbe")
	} else {
		builder.addPostUserArgs("-target", builder.target.target)
	}
	return nil
}

// Return the a directory which contains an 'ld' that gcc is using.
func getLinkerPath(env env, linkerCmd string, rootPath string) string {
	// We did not pass the tuple i686-pc-linux-gnu to x86-32 clang. Instead,
	// we passed '-m32' to clang. As a result, clang does not want to use the
	// i686-pc-linux-gnu-ld, so we need to add this to help clang find the right
	// linker.
	for _, path := range strings.Split(env.getenv("PATH"), ":") {
		if linkerPath, err := filepath.EvalSymlinks(filepath.Join(path, linkerCmd)); err == nil {
			return filepath.Dir(linkerPath)
		}
	}

	// When using the sdk outside chroot, we need to provide the cross linker path
	// to the compiler via -B ${linker_path}. This is because for gcc, it can
	// find the right linker via searching its internal paths. Clang does not have
	// such feature, and it falls back to $PATH search only. However, the path of
	// ${SDK_LOCATION}/bin is not necessarily in the ${PATH}. To fix this, we
	// provide the directory that contains the cross linker wrapper to clang.
	// Outside chroot, it is the top bin directory form the sdk tarball.
	return filepath.Join(rootPath, "bin")
}

func hasAtLeastOnePrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
