#!/usr/bin/python

# Copyright (c) 2011 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import StringIO
import unittest
from experiment_file import ExperimentFile

EXPERIMENT_FILE_1 = """
  board: x86-alex
  remote: chromeos-alex3

  benchmark: PageCycler {
    iterations: 3
  }

  image1 {
    chromeos_image: /usr/local/google/cros_image1.bin
  }

  image2 {
    chromeos_image: /usr/local/google/cros_image2.bin
  }
  """

EXPERIMENT_FILE_2 = """
  board: x86-alex
  remote: chromeos-alex3
  iterations: 3

  benchmark: PageCycler {
  }

  benchmark: AndroidBench {
    iterations: 2
  }

  image1 {
    chromeos_image:/usr/local/google/cros_image1.bin
  }

  image2 {
    chromeos_image: /usr/local/google/cros_image2.bin
  }
  """

EXPERIMENT_FILE_3 = """
  board: x86-alex
  remote: chromeos-alex3
  iterations: 3

  benchmark: PageCycler {
  }

  image1 {
    chromeos_image:/usr/local/google/cros_image1.bin
  }

  image1 {
    chromeos_image: /usr/local/google/cros_image2.bin
  }
  """


class ExperimentFileTest(unittest.TestCase):
  def testLoadExperimentFile1(self):
    input_file = StringIO.StringIO(EXPERIMENT_FILE_1)
    experiment_file = ExperimentFile(input_file)
    global_settings = experiment_file.GetGlobalSettings()
    self.assertEqual(global_settings.GetField("remote"), ["chromeos-alex3"])

    benchmark_settings = experiment_file.GetSettings("benchmark")
    self.assertEqual(len(benchmark_settings), 1)
    self.assertEqual(benchmark_settings[0].name, "PageCycler")
    self.assertEqual(benchmark_settings[0].GetField("iterations"), 3)

    label_settings = experiment_file.GetSettings("label")
    self.assertEqual(len(label_settings), 2)
    self.assertEqual(label_settings[0].name, "image1")
    self.assertEqual(label_settings[0].GetField("board"), "x86-alex")
    self.assertEqual(label_settings[0].GetField("chromeos_image"),
                     "/usr/local/google/cros_image1.bin")

  def testOverrideSetting(self):
    input_file = StringIO.StringIO(EXPERIMENT_FILE_2)
    experiment_file = ExperimentFile(input_file)
    global_settings = experiment_file.GetGlobalSettings()
    self.assertEqual(global_settings.GetField("remote"), ["chromeos-alex3"])

    benchmark_settings = experiment_file.GetSettings("benchmark")
    self.assertEqual(len(benchmark_settings), 2)
    self.assertEqual(benchmark_settings[0].name, "PageCycler")
    self.assertEqual(benchmark_settings[0].GetField("iterations"), 3)
    self.assertEqual(benchmark_settings[1].name, "AndroidBench")
    self.assertEqual(benchmark_settings[1].GetField("iterations"), 2)

  def testDuplicateLabel(self):
    input_file = StringIO.StringIO(EXPERIMENT_FILE_3)
    self.assertRaises(Exception, ExperimentFile, input_file)


if __name__ == "__main__":
  unittest.main()
