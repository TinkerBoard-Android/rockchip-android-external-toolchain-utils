import threading
from automation.common import job
from utils import utils
import re
from utils import logger
from utils import command_executer

JOBDIR_PREFIX = "/usr/local/google/tmp/automation/job-"

class JobExecuter(threading.Thread):

  def __init__(self, job, machines, job_manager):
    threading.Thread.__init__(self)
    self.cmd_executer = command_executer.GetCommandExecuter()
    self.job = job
    self.job_manager = job_manager
    self.machines = machines
    self.command_terminator = command_executer.CommandTerminator()


  def _IsJobFailed(self, return_value, fail_message):
    if return_value == 0:
      return False
    else:
      logger.GetLogger().LogError("Job failed. Exit code %s. %s"
                                  % (return_value, fail_message))
      if self.command_terminator.IsTerminated():
        logger.GetLogger().LogOutput("Job '%s' was killed"
                                     % str(self.job.GetID()))
      self.job_manager.NotifyJobComplete(self.job, job.STATUS_FAILED)
      return True


  def _FormatCommand(self, command):
    ret = command
    ret.replace("$JOB_ID", str(self.job.GetID()))
    ret.replace("$PRIMARY_MACHINE", self.job.machines[0].name)
    mo = re.search("SECONDARY_MACHINES\[(\d+)\]", ret)
    if mo is not None:
      index = int(mo.group(1))
      ret = (ret[0:mo.start()] + self.job.machines[1+index] +
             ret[mo.end():])
    return ret

  def Kill(self):
    self.command_terminator.Terminate()


  def run(self):
    # Set job directory
    job_dir = JOBDIR_PREFIX + str(self.job.GetID())
    self.job.SetJobDir(job_dir)

    primary_machine = self.machines[0]
    self.job.SetMachines(self.machines)

    logger.GetLogger().LogOutput("Executing job with ID '%s' on machine '%s' "
                                 "in directory '%s'" %
                                 (self.job.GetID(), self.job.GetMachine().name,
                                  self.job.GetJobDir()))

    rm_success = self.cmd_executer.RunCommand("sudo rm -rf %s" %
                                              self.job.GetJobDir(),
                                              False, primary_machine.name,
                                              primary_machine.username,
                                              self.command_terminator)
    if self._IsJobFailed(rm_success, "rm of old job directory Failed."):
      return

    mkdir_success = self.cmd_executer.RunCommand("mkdir -p %s" %
                                                 self.job.GetWorkDir(),
                                                 False, primary_machine.name,
                                                 primary_machine.username,
                                                 self.command_terminator)
    if self._IsJobFailed(mkdir_success, "mkdir of new job directory Failed."):
      return

    for required_folder in self.job.GetRequiredFolders():
      to_folder = self.job.GetWorkDir() + "/" + required_folder.dest
      from_folder = (required_folder.job.GetWorkDir() + "/" +
                     required_folder.src)

      to_folder_parent = utils.GetRoot(to_folder)[0]
      self.cmd_executer.RunCommand("mkdir -p %s"  %
                                   to_folder_parent,
                                   False, primary_machine.name,
                                   primary_machine.username,
                                   self.command_terminator)

      from_machine = required_folder.job.GetMachine().name
      from_user = required_folder.job.GetMachine().username
      to_machine = self.job.GetMachine().name
      to_user = self.job.GetMachine().username
      if from_machine == to_machine and required_folder.read_only:
        # No need to make a copy, just symlink it
        symlink_success = self.cmd_executer.RunCommand("ln -sf %s %s" %
                                                       (from_folder, to_folder),
                                                       False,
                                                       from_machine, from_user,
                                                       self.command_terminator)
        if self._IsJobFailed(symlink_success, "Failed to create symlink to "
                             "required directory."):
          return
      else:
        copy_success = self.cmd_executer.CopyFiles(from_folder, to_folder,
                                                   from_machine, to_machine,
                                                   from_user, to_user, True)
        if self._IsJobFailed(copy_success, "Failed to copy required files."):
          return

    command = self.job.GetCommand()

    command = self._FormatCommand(command)

    command_success = (self.cmd_executer.
                       RunCommand("PS1=. TERM=linux "
                                  "source ~/.bashrc ; cd %s && %s"
                                  % (self.job.GetWorkDir(), command), False,
                                  primary_machine.name,
                                  primary_machine.username,
                                  self.command_terminator))

    if self._IsJobFailed(command_success,
                         "Command failed to execute: '%s'." % command):
      return

    # If we get here, the job succeeded. 
    logger.GetLogger().LogOutput("Job completed successfully.")
    self.job_manager.NotifyJobComplete(self.job, job.STATUS_COMPLETED)



