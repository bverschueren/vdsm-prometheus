%global provider        github
%global provider_tld    com
%global project         rmohr
%global repo            vdsm-prometheus
# https://github.com/rmohr/vdsm-prometheus
%global provider_prefix %{provider}.%{provider_tld}/%{project}/%{repo}
%global import_path     %{provider_prefix}

Name:           %{repo}
Version:        0.0.3
Release:        1%{?dist}
Summary:        Go client for VDSM which exposes host and VM stats as prometheus metrics
License:        GPLv3+
URL:            https://%{provider_prefix}
Source0:        will be filled by tito

# e.g. el6 has ppc64 arch without gcc-go, so EA tag is required
ExclusiveArch:  %{?go_arches:%{go_arches}}%{!?go_arches:%{ix86} x86_64 %{arm}}
# If go_compiler is not set to 1, there is no virtual provide. Use golang instead.
BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires: systemd-units
BuildRequires: git
Requires: vdsm
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description
%{summary}

%prep
%setup -q -n %{repo}
mkdir -p %{_builddir}/go/%{import_path}
cp -a ./* %{_builddir}/go/%{import_path}/
echo $PWD
echo %{_builddir}

%build
export GOPATH=%{_builddir}/go/:%{gopath}
cd %{_builddir}/go/%{import_path}/
go get -d
go test
go build -a -ldflags "-B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n')" -v -x
cd -
cp %{_builddir}/go/%{import_path}/%{name} .

%install
install -D -p -m 755 %{name} %{buildroot}%{_bindir}/%{name}
install -D -p %{name}.service %{buildroot}%{_unitdir}/%{name}.service
install -D -p 10-%{name}.conf %{buildroot}%{_unitdir}/%{name}.service.d/10-%{name}.conf

%check

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%postun
%systemd_postun %{name}.service

%files
%doc README.md
%license LICENSE
%{_bindir}/%{name}
%config %{_unitdir}/%{name}.service
%config %{_unitdir}/%{name}.service.d

%changelog
* Thu Mar 24 2016 Roman Mohr <rmohr@redhat.com> 0.0.3-1
- new package built with tito

* Tue Mar 22 2016 Roman Mohr <rmohr@redhat.com> 0.0.2-1
- new package built with tito

* Mon Mar 21 2016 Roman Mohr <roman@fenkhuber.at> - 0-0.1-1
- First package for Fedora
