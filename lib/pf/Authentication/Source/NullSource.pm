package pf::Authentication::Source::NullSource;
=head1 NAME

pf::Authentication::Source::NullSource add documentation

=cut

=head1 DESCRIPTION

pf::Authentication::Source::NullSource

=cut

use strict;
use warnings;
use Moose;
use pf::config qw($FALSE $TRUE);
use Email::Valid;

extends 'pf::Authentication::Source';

has '+class' => (default => 'exclusive');
has '+type' => (default => 'Null');
has '+unique' => (default => 1);
has 'email_required' => ( is=> 'rw', default => 0);

sub match_in_subclass {
    my ($self, $params, $rule, $own_conditions, $matching_conditions) = @_;
    return $params->{'username'};
}

=head2 authenticate

=cut

sub authenticate {
    my ($self, $username, $password) = @_;
    if (!$self->email_required || Email::Valid->address($username) ) {
        return ($TRUE, 'Successful authentication using null source.');
    }
    return ($FALSE, 'Successful authentication using null source.');
}

=head1 AUTHOR

Inverse inc. <info@inverse.ca>

=head1 COPYRIGHT

Copyright (C) 2005-2013 Inverse inc.

=head1 LICENSE

This program is free software; you can redistribute it and::or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301,
USA.

=cut

1;

