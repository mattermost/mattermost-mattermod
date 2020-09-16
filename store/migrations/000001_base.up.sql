BEGIN;

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `Issues`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE IF NOT EXISTS `Issues` (
  `RepoOwner` varchar(128) NOT NULL,
  `RepoName` varchar(128) NOT NULL,
  `Number` int(11) NOT NULL,
  `Username` varchar(128) DEFAULT NULL,
  `State` varchar(8) DEFAULT NULL,
  `Labels` text,
  PRIMARY KEY (`RepoOwner`,`RepoName`,`Number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `PullRequests`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE IF NOT EXISTS `PullRequests` (
  `RepoOwner` varchar(128) NOT NULL,
  `RepoName` varchar(128) NOT NULL,
  `FullName` varchar(2083) DEFAULT '',
  `Number` int(11) NOT NULL,
  `Username` varchar(128) DEFAULT NULL,
  `Ref` varchar(128) DEFAULT NULL,
  `Sha` varchar(48) DEFAULT NULL,
  `Labels` text,
  `State` varchar(8) DEFAULT NULL,
  `BuildStatus` varchar(8) DEFAULT NULL,
  `BuildConclusion` varchar(20) DEFAULT NULL,
  `BuildLink` text,
  `URL` varchar(2083) DEFAULT '',
  `CreatedAt` timestamp NULL DEFAULT NULL,
  `MaintainerCanModify` tinyint(1) DEFAULT NULL,
  `Merged` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`RepoOwner`,`RepoName`,`Number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `Spinmint`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE IF NOT EXISTS `Spinmint` (
  `InstanceId` varchar(128) NOT NULL,
  `RepoOwner` varchar(255) DEFAULT NULL,
  `RepoName` varchar(255) DEFAULT NULL,
  `Number` int(11) DEFAULT NULL,
  `CreatedAt` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`InstanceId`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

COMMIT;
