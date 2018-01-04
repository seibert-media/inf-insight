def label = "buildpod.${env.JOB_NAME}".replaceAll(/[^A-Za-z-_\.]+/, '_').take(62) + "p"

podTemplate(
	name: label,
	label: label,
	containers: [
		containerTemplate(
			name: 'jnlp',
			image: 'eu.gcr.io/smedia-kubernetes/jenkins-slave-debian:1.3.0',
			args: '${computer.jnlpmac} ${computer.name}',
			resourceRequestCpu: '200m',
			resourceRequestMemory: '600Mi',
			resourceLimitCpu: '400m',
			resourceLimitMemory: '600Mi',
		),
		containerTemplate(
			name: 'build-golang',
			image: 'eu.gcr.io/smedia-kubernetes/build-golang:1.9.1-1.0.0',
			ttyEnabled: true,
			command: 'cat',
			resourceRequestCpu: '500m',
			resourceRequestMemory: '500Mi',
			resourceLimitCpu: '2000m',
			resourceLimitMemory: '500Mi',
		),
		containerTemplate(
			name: 'build-docker',
			image: 'eu.gcr.io/smedia-kubernetes/build-docker:1.0.3',
			ttyEnabled: true,
			command: 'cat',
			privileged: true,
			resourceRequestCpu: '500m',
			resourceRequestMemory: '500Mi',
			resourceLimitCpu: '2000m',
			resourceLimitMemory: '500Mi',
		),
	],
	volumes: [
		secretVolume(mountPath: '/root/.ssh', secretName: 'ssh'),
		secretVolume(mountPath: '/home/jenkins/.docker', secretName: 'docker-quay'),
		hostPathVolume(hostPath: '/var/run/docker.sock', mountPath: '/var/run/docker.sock'),
	],
	inheritFrom: '',
	namespace: 'jenkins',
	nodeSelector: 'cloud.google.com/gke-preemptible=true',
	serviceAccount: '',
	workspaceVolume: emptyDirWorkspaceVolume(false),
) {
	node(label) {
		properties([
			buildDiscarder(logRotator(artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '14', numToKeepStr: '50')),
			pipelineTriggers([
				cron('H 2 * * *'),
				pollSCM('H/5 * * * *'),
			]),
		])
		try {
			container('build-golang') {
				stage('Fix SSH Permissions') {
					timeout(time: 1, unit: 'MINUTES') {
						sh 'chmod 600 /root/.ssh/*'
					}
				}
				stage('Checkout') {
					timeout(time: 5, unit: 'MINUTES') {
						checkout scm
						sh """
						mkdir -p /go/src/github.com/seibert-media
						ln -s `pwd` /go/src/github.com/seibert-media/inf-insight
						"""
					}
				}
				stage('Deps') {
					timeout(time: 15, unit: 'MINUTES') {
						sh "cd /go/src/github.com/seibert-media/inf-insight && make deps"
					}
				}
				stage('Test') {
					timeout(time: 15, unit: 'MINUTES') {
						sh "cd /go/src/github.com/seibert-media/inf-insight && make test"
					}
				}
			}
			container('build-docker') {
				stage('Fix SSH Permissions') {
					timeout(time: 1, unit: 'MINUTES') {
						sh 'chmod 600 /root/.ssh/*'
					}
				}
				stage('Checkout') {
					timeout(time: 5, unit: 'MINUTES') {
						checkout scm
						sh """
						mkdir -p /go/src/github.com/seibert-media
						ln -s `pwd` /go/src/github.com/seibert-media/inf-insight
						"""
					}
				}
				stage('Build') {
					timeout(time: 15, unit: 'MINUTES') {
						sh "cd /go/src/github.com/seibert-media/inf-insight && make docker"
					}
				}
				stage('Upload') {
					timeout(time: 15, unit: 'MINUTES') {
						sh "cd /go/src/github.com/seibert-media/inf-insight && make upload"
					}
				}
			}
			currentBuild.result = 'SUCCESS'
		} catch (any) {
			currentBuild.result = 'FAILURE'
			throw any //rethrow exception to prevent the build from proceeding
		} finally {
			step([$class: 'Mailer',
				notifyEveryUnstableBuild: false,
				recipients: '',
				sendToIndividuals: true])
		}
	}
}
