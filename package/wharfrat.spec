Name:           wharfrat
Version:        @@VERSION@@
Release:        @@RELEASE@@
Summary:        container based development environments

License:        MIT
URL:            https://wharfr.at

%description
wharfrat uses docker to manage developement environemnts using version
controlled configuration files.

%build

%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/usr/share/licenses/wharfrat
cp /wharfrat/LICENSE $RPM_BUILD_ROOT/usr/share/licenses/wharfrat/
mkdir -p $RPM_BUILD_ROOT/%{_bindir}
install /wharfrat/dist/wharfrat $RPM_BUILD_ROOT/%{_bindir}
ln -s wharfrat $RPM_BUILD_ROOT/%{_bindir}/wr
ln -s wharfrat $RPM_BUILD_ROOT/%{_bindir}/wr-exec


%files
%license /usr/share/licenses/wharfrat/LICENSE
#%doc add-docs-here
%{_bindir}/*



%changelog
* Sun Jul 29 2018 jp3
- 