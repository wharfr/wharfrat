*************
Configuration
*************

.. contents:: Table of Contents

Project Configuration
=====================

Local Configuration
===================

In addition to the shared project configuration each user can have a local
configuration. This configuration allows changing the Docker URL, and adding
extra steps to the container setup.

.. code-block:: toml

  docker-url = "file:///var/run/docker.sock"

  [[setups]]
      project = ".*/test"
      setup-prep = """
          echo "LOCAL PREP: $*"
          pwd
      """

      setup-pre = """
          echo "LOCAL PRE"
          pwd
      """

      setup-post = """
          echo "LOCAL POST"
      """

      [setups.tarballs]
          "path/to/tarball.tgz" = "/path/in/container/to/unpack"

      [setups.env]
          "LOCAL_CRATE_ENV" = "true"

  [[setups]]
      setup-prep = """
          echo "LOCAL PREP: $*"
          pwd
      """

      setup-pre = """
          echo "LOCAL PRE"
          pwd
      """

      setup-post = """
          echo "LOCAL POST"
      """

      [setups.env]
          "LOCAL_CRATE_ENV" = "true"

The available settings are:

+------------+-----------------------------------------------------------------+
| docker-url | The URL to use to connect to Docker                             |
+------------+------------+----------------------------------------------------+
| setups     | project    | a regular expression that much match the project   |
|            |            | path for this setup to be applies. If not          |
|            |            | specified, then ".*" is used.                      |
|            +------------+----------------------------------------------------+
|            | crate      | a regular expression that must match the crate     |
|            |            | name for this setup to be applied. If not          |
|            |            | specified, then ".*" is used.                      |
|            +------------+----------------------------------------------------+
|            | setup-prep | script to run locally before doing anything else   |
|            +------------+----------------------------------------------------+
|            | setup-pre  | script to run remotely before unpacking tarballs   |
|            +------------+----------------------------------------------------+
|            | setup-post | script to run remotely after unpacking tarballs    |
|            +------------+----------------------------------------------------+
|            | tarballs   | a table to tarballs to be unpacked into the        |
|            |            | container, mapping tarball path to target path in  |
|            |            | the container                                      |
|            +------------+----------------------------------------------------+
|            | env        | a table of environment variables to set in the     |
|            |            | container, mapping name to value                   |
+------------+------------+----------------------------------------------------+
